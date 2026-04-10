using System.Threading;
using MegaCrit.Sts2.Core.Logging;
using Spire2Mind.Bridge.Game.Threading;
using Spire2Mind.Bridge.Game.Hooks;
using Spire2Mind.Bridge.Game.Util;
using Spire2Mind.Bridge.Http;

namespace Spire2Mind.Bridge;

internal static class BridgeRuntime
{
    private static int _started;
    private static int _stateReadsEnabled;
    private static long _lastGameFrameTicks;
    private static long _eventSequence;

    private static HttpServer? _server;
    private static EventService? _events;

    public static bool IsReady => (_server?.IsRunning ?? false) && StateReadsEnabled;

    public static bool StateReadsEnabled => Volatile.Read(ref _stateReadsEnabled) == 1;

    public static DateTime? LastGameFrameUtc
    {
        get
        {
            var ticks = Interlocked.Read(ref _lastGameFrameTicks);
            return ticks == 0 ? null : new DateTime(ticks, DateTimeKind.Utc);
        }
    }

    public static int Port => _server?.Port ?? BridgeDefaults.DefaultPort;

    internal static long EventSequence => Interlocked.Read(ref _eventSequence);

    public static EventService? Events => _events;

    internal static long NextEventSequence()
    {
        return Interlocked.Increment(ref _eventSequence);
    }

    public static void EnsureStarted()
    {
        if (Interlocked.Exchange(ref _started, 1) == 1)
        {
            return;
        }

        try
        {
            GameThread.Initialize();

            _server = new HttpServer(BridgeRouter.HandleAsync);
            _server.Start();
            _events = new EventService();
            _events.Start();

            Log.Info($"[{Entry.ModId}] HTTP bridge listening on http://127.0.0.1:{_server.Port}/");
        }
        catch (Exception ex)
        {
            Interlocked.Exchange(ref _started, 0);
            Log.Error($"[{Entry.ModId}] Failed to start bridge runtime: {ex}");
            throw;
        }
    }

    public static void Stop()
    {
        try
        {
            _events?.Dispose();
            _server?.Dispose();
        }
        catch (Exception ex)
        {
            Log.Error($"[{Entry.ModId}] Failed to stop HTTP server cleanly: {ex}");
        }
        finally
        {
            _events = null;
            _server = null;
            Interlocked.Exchange(ref _stateReadsEnabled, 0);
            Interlocked.Exchange(ref _lastGameFrameTicks, 0);
            Interlocked.Exchange(ref _eventSequence, 0);
            Interlocked.Exchange(ref _started, 0);
        }
    }

    public static void NotifyGameFrame(bool stateReadsEnabled)
    {
        Interlocked.Exchange(ref _lastGameFrameTicks, DateTime.UtcNow.Ticks);
        if (stateReadsEnabled)
        {
            Interlocked.Exchange(ref _stateReadsEnabled, 1);
        }
    }
}
