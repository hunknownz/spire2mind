using System.Collections.Concurrent;
using System.Diagnostics;
using System.Threading;
using MegaCrit.Sts2.Core.Nodes;

namespace Spire2Mind.Bridge.Game;

internal sealed class GameThreadContext : SynchronizationContext
{
    private readonly ConcurrentQueue<(SendOrPostCallback Callback, object? State)> _queue = new();

    private readonly int _gameThreadId;

    public GameThreadContext()
    {
        _gameThreadId = Environment.CurrentManagedThreadId;
    }

    public bool IsGameThread => Environment.CurrentManagedThreadId == _gameThreadId;

    public override void Post(SendOrPostCallback d, object? state)
    {
        _queue.Enqueue((d, state));
    }

    public void ProcessFrame()
    {
        var stopwatch = Stopwatch.StartNew();
        BridgeRuntime.NotifyGameFrame(NGame.Instance?.MainMenu != null);

        while (_queue.TryDequeue(out var work))
        {
            work.Callback(work.State);

            if (stopwatch.ElapsedMilliseconds > 8)
            {
                break;
            }
        }
    }
}
