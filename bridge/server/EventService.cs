using System.Net;
using System.Text;
using System.Threading.Channels;
using Spire2Mind.Bridge.Game.State;
using Spire2Mind.Bridge.Game.Threading;
using Spire2Mind.Bridge.Game.Util;
using Spire2Mind.Bridge.Models;

namespace Spire2Mind.Bridge.Http;

internal sealed class EventService : IDisposable
{
    private readonly object _sync = new();
    private readonly CancellationTokenSource _cts = new();
    private readonly List<Channel<BridgeEvent>> _subscribers = new();

    private Task? _pollLoop;
    private StateDigest? _lastDigest;

    /// <summary>
    /// Signal used by BridgeHooker to wake the poll loop immediately
    /// instead of waiting for the next timer tick.
    /// </summary>
    private readonly SemaphoreSlim _hookSignal = new(0);

    public void Start()
    {
        lock (_sync)
        {
            if (_pollLoop != null)
            {
                return;
            }

            BridgeHooker.OnHookFired += OnHookFired;
            _pollLoop = Task.Run(() => PollLoopAsync(_cts.Token), _cts.Token);
        }
    }

    public EventSubscription Subscribe()
    {
        var channel = Channel.CreateBounded<BridgeEvent>(new BoundedChannelOptions(BridgeDefaults.EventBufferCapacity)
        {
            SingleReader = true,
            SingleWriter = false,
            FullMode = BoundedChannelFullMode.DropOldest
        });

        lock (_sync)
        {
            _subscribers.Add(channel);
        }

        PublishTo(
            channel,
            new BridgeEvent
            {
                Type = "stream_ready",
                Sequence = NextSequence(),
                TimestampUtc = DateTime.UtcNow,
                RunId = _lastDigest?.RunId ?? "run_unknown",
                Screen = _lastDigest?.Screen ?? ScreenIds.Unknown,
                InCombat = _lastDigest?.InCombat ?? false,
                Turn = _lastDigest?.Turn,
                AvailableActions = _lastDigest?.AvailableActions ?? Array.Empty<string>(),
                Data = new
                {
                    ready = BridgeRuntime.IsReady,
                    stateReadsEnabled = BridgeRuntime.StateReadsEnabled,
                    lastGameFrameUtc = BridgeRuntime.LastGameFrameUtc
                }
            });

        return new EventSubscription(channel.Reader, () => Remove(channel));
    }

    public async Task StreamAsync(HttpListenerContext context, string requestId, CancellationToken cancellationToken)
    {
        using var subscription = Subscribe();

        var response = context.Response;
        response.StatusCode = 200;
        response.SendChunked = true;
        response.KeepAlive = true;
        response.ContentType = "text/event-stream; charset=utf-8";
        response.Headers["Cache-Control"] = "no-cache";
        response.Headers["Connection"] = "keep-alive";
        response.Headers["X-Accel-Buffering"] = "no";

        await WriteCommentAsync(response, $"request_id={requestId}", cancellationToken).ConfigureAwait(false);

        try
        {
            while (!cancellationToken.IsCancellationRequested)
            {
                var readTask = subscription.Reader.ReadAsync(cancellationToken).AsTask();
                var keepAliveTask = Task.Delay(BridgeDefaults.EventKeepAliveInterval, cancellationToken);
                var completed = await Task.WhenAny(readTask, keepAliveTask).ConfigureAwait(false);

                if (completed == readTask)
                {
                    var bridgeEvent = await readTask.ConfigureAwait(false);
                    await WriteEventAsync(response, bridgeEvent, cancellationToken).ConfigureAwait(false);
                    continue;
                }

                await WriteCommentAsync(response, $"keepalive {DateTime.UtcNow:o}", cancellationToken).ConfigureAwait(false);
            }
        }
        catch (OperationCanceledException)
        {
            // Client disconnected or server is shutting down.
        }
        catch (HttpListenerException)
        {
            // Connection closed by peer.
        }
        catch (IOException)
        {
            // Connection closed by peer.
        }
    }

    public void Dispose()
    {
        BridgeHooker.OnHookFired -= OnHookFired;
        _cts.Cancel();

        Task? pollLoop;
        lock (_sync)
        {
            pollLoop = _pollLoop;
            _pollLoop = null;

            foreach (var subscriber in _subscribers)
            {
                subscriber.Writer.TryComplete();
            }

            _subscribers.Clear();
        }

        try
        {
            pollLoop?.Wait(TimeSpan.FromSeconds(2));
        }
        catch
        {
            // Best effort only.
        }

        _hookSignal.Dispose();
    }

    private async Task PollLoopAsync(CancellationToken cancellationToken)
    {
        using var timer = new PeriodicTimer(BridgeDefaults.EventPollInterval);

        while (await timer.WaitForNextTickAsync(cancellationToken).ConfigureAwait(false))
        {
            if (!BridgeRuntime.StateReadsEnabled)
            {
                continue;
            }

            BridgeStateSnapshot snapshot;
            try
            {
                snapshot = await GameThread.InvokeAsync(StateSnapshotBuilder.Build).ConfigureAwait(false);
            }
            catch (Exception ex) when (ex is not OperationCanceledException)
            {
                MegaCrit.Sts2.Core.Logging.Log.Warn($"[{Entry.ModId}] Event poll state build failed: {ex.GetType().Name}: {ex.Message}");
                continue;
            }

            PublishChanges(snapshot);
        }
    }

    private void OnHookFired(string hookName, object? data)
    {
        // Publish a lightweight hook-specific event immediately
        Publish(new BridgeEvent
        {
            Type = $"hook:{hookName}",
            Sequence = NextSequence(),
            TimestampUtc = DateTime.UtcNow,
            RunId = _lastDigest?.RunId ?? "run_unknown",
            Screen = _lastDigest?.Screen ?? ScreenIds.Unknown,
            InCombat = _lastDigest?.InCombat ?? false,
            Turn = _lastDigest?.Turn,
            AvailableActions = _lastDigest?.AvailableActions ?? Array.Empty<string>(),
            Data = data
        });

        // Wake the poll loop to rebuild full state
        try
        {
            _hookSignal.Release();
        }
        catch (SemaphoreFullException)
        {
            // Already signaled, no problem
        }
    }

    private void PublishChanges(BridgeStateSnapshot snapshot)
    {
        var digest = StateDigest.FromSnapshot(snapshot);
        var previousDigest = _lastDigest;
        _lastDigest = digest;

        if (previousDigest == null || !string.Equals(previousDigest.RunId, digest.RunId, StringComparison.Ordinal))
        {
            Publish(
                BuildEvent(
                    "session_started",
                    snapshot,
                    new
                    {
                        previousRunId = previousDigest?.RunId
                    }));
        }

        if (previousDigest == null)
        {
            Publish(BuildEvent("state_changed", snapshot, null));
            return;
        }

        if (!string.Equals(previousDigest.Screen, digest.Screen, StringComparison.Ordinal))
        {
            Publish(
                BuildEvent(
                    "screen_changed",
                    snapshot,
                    new
                    {
                        previousScreen = previousDigest.Screen,
                        screen = digest.Screen
                    }));
        }

        if (previousDigest.InCombat != digest.InCombat)
        {
            Publish(BuildEvent(digest.InCombat ? "combat_started" : "combat_ended", snapshot, null));
        }

        if (digest.InCombat && previousDigest.Turn != digest.Turn)
        {
            Publish(
                BuildEvent(
                    "combat_turn_changed",
                    snapshot,
                    new
                    {
                        previousTurn = previousDigest.Turn,
                        turn = digest.Turn
                    }));
        }

        if (!string.Equals(previousDigest.ActionSignature, digest.ActionSignature, StringComparison.Ordinal))
        {
            Publish(
                BuildEvent(
                    "available_actions_changed",
                    snapshot,
                    new
                    {
                        previousActions = previousDigest.AvailableActions,
                        availableActions = digest.AvailableActions
                    }));
        }

        if (!previousDigest.HasActions && digest.HasActions)
        {
            Publish(BuildEvent("player_action_window_opened", snapshot, null));
        }
        else if (previousDigest.HasActions && !digest.HasActions)
        {
            Publish(BuildEvent("player_action_window_closed", snapshot, null));
        }

        if (!previousDigest.Matches(digest))
        {
            Publish(BuildEvent("state_changed", snapshot, null));
        }
    }

    private BridgeEvent BuildEvent(string eventType, BridgeStateSnapshot snapshot, object? data)
    {
        return new BridgeEvent
        {
            Type = eventType,
            Sequence = NextSequence(),
            TimestampUtc = DateTime.UtcNow,
            RunId = snapshot.RunId,
            Screen = snapshot.Screen,
            InCombat = snapshot.InCombat,
            Turn = snapshot.Turn,
            AvailableActions = snapshot.AvailableActions,
            Data = data
        };
    }

    private void Publish(BridgeEvent bridgeEvent)
    {
        List<Channel<BridgeEvent>> subscribers;
        lock (_sync)
        {
            subscribers = _subscribers.ToList();
        }

        foreach (var subscriber in subscribers)
        {
            PublishTo(subscriber, bridgeEvent);
        }
    }

    private static void PublishTo(Channel<BridgeEvent> channel, BridgeEvent bridgeEvent)
    {
        if (!channel.Writer.TryWrite(bridgeEvent))
        {
            channel.Writer.TryComplete();
        }
    }

    private void Remove(Channel<BridgeEvent> channel)
    {
        lock (_sync)
        {
            _subscribers.Remove(channel);
        }

        channel.Writer.TryComplete();
    }

    private static async Task WriteCommentAsync(HttpListenerResponse response, string comment, CancellationToken cancellationToken)
    {
        var payload = Encoding.UTF8.GetBytes($": {comment}\n\n");
        await response.OutputStream.WriteAsync(payload, 0, payload.Length, cancellationToken).ConfigureAwait(false);
        await response.OutputStream.FlushAsync(cancellationToken).ConfigureAwait(false);
    }

    private static async Task WriteEventAsync(HttpListenerResponse response, BridgeEvent bridgeEvent, CancellationToken cancellationToken)
    {
        var json = JsonHelper.SerializeToUtf8(bridgeEvent);
        var prefix = Encoding.UTF8.GetBytes("event: bridge_event\ndata: ");
        var suffix = Encoding.UTF8.GetBytes("\n\n");

        await response.OutputStream.WriteAsync(prefix, 0, prefix.Length, cancellationToken).ConfigureAwait(false);
        await response.OutputStream.WriteAsync(json, 0, json.Length, cancellationToken).ConfigureAwait(false);
        await response.OutputStream.WriteAsync(suffix, 0, suffix.Length, cancellationToken).ConfigureAwait(false);
        await response.OutputStream.FlushAsync(cancellationToken).ConfigureAwait(false);
    }

    private long NextSequence()
    {
        return BridgeRuntime.NextEventSequence();
    }

    internal sealed class EventSubscription : IDisposable
    {
        private readonly Action _dispose;

        private int _disposed;

        public EventSubscription(ChannelReader<BridgeEvent> reader, Action dispose)
        {
            Reader = reader;
            _dispose = dispose;
        }

        public ChannelReader<BridgeEvent> Reader { get; }

        public void Dispose()
        {
            if (Interlocked.Exchange(ref _disposed, 1) == 1)
            {
                return;
            }

            _dispose();
        }
    }

    private sealed record StateDigest(
        string RunId,
        string Screen,
        bool InCombat,
        int? Turn,
        IReadOnlyList<string> AvailableActions,
        string ActionSignature)
    {
        public bool HasActions => AvailableActions.Count > 0;

        public bool Matches(StateDigest other)
        {
            return string.Equals(RunId, other.RunId, StringComparison.Ordinal) &&
                   string.Equals(Screen, other.Screen, StringComparison.Ordinal) &&
                   InCombat == other.InCombat &&
                   Turn == other.Turn &&
                   string.Equals(ActionSignature, other.ActionSignature, StringComparison.Ordinal);
        }

        public static StateDigest FromSnapshot(BridgeStateSnapshot snapshot)
        {
            return new StateDigest(
                snapshot.RunId,
                snapshot.Screen,
                snapshot.InCombat,
                snapshot.Turn,
                snapshot.AvailableActions.ToArray(),
                string.Join("|", snapshot.AvailableActions));
        }
    }
}

internal sealed class BridgeEvent
{
    public string Type { get; init; } = "state_changed";

    public long Sequence { get; init; }

    public DateTime TimestampUtc { get; init; }

    public string RunId { get; init; } = "run_unknown";

    public string Screen { get; init; } = ScreenIds.Unknown;

    public bool InCombat { get; init; }

    public int? Turn { get; init; }

    public IReadOnlyList<string> AvailableActions { get; init; } = Array.Empty<string>();

    public object? Data { get; init; }
}
