using System.Threading;
using MegaCrit.Sts2.Core.Nodes;

namespace Spire2Mind.Bridge.Game.Threading;

internal static class GameThread
{
    private static readonly object Sync = new();

    private static GameThreadContext? _context;

    public static void Initialize()
    {
        lock (Sync)
        {
            if (_context != null)
            {
                return;
            }

            ArgumentNullException.ThrowIfNull(NGame.Instance);

            _context = new GameThreadContext();
            // Keep the dispatcher private to the Bridge so we do not hijack the
            // game's ambient async continuation behavior.
            NGame.Instance.GetTree().ProcessFrame += _context.ProcessFrame;
        }
    }

    public static Task<T> InvokeAsync<T>(Func<T> action)
    {
        ArgumentNullException.ThrowIfNull(action);
        EnsureInitialized();

        if (_context!.IsGameThread)
        {
            return Task.FromResult(action());
        }

        var tcs = new TaskCompletionSource<T>(TaskCreationOptions.RunContinuationsAsynchronously);
        _context.Post(
            _ =>
            {
                try
                {
                    tcs.TrySetResult(action());
                }
                catch (Exception ex)
                {
                    tcs.TrySetException(ex);
                }
            },
            null);

        return tcs.Task;
    }

    public static Task<T> InvokeAsync<T>(Func<Task<T>> action)
    {
        ArgumentNullException.ThrowIfNull(action);
        EnsureInitialized();

        if (_context!.IsGameThread)
        {
            return RunWithContextAsync(action);
        }

        var tcs = new TaskCompletionSource<T>(TaskCreationOptions.RunContinuationsAsynchronously);
        _context.Post(_ => _ = InvokeAsyncCore(action, tcs), null);
        return tcs.Task;
    }

    private static void EnsureInitialized()
    {
        if (_context == null)
        {
            throw new InvalidOperationException("GameThread.Initialize() must run before using the game thread dispatcher.");
        }
    }

    private static async Task InvokeAsyncCore<T>(Func<Task<T>> action, TaskCompletionSource<T> tcs)
    {
        try
        {
            tcs.TrySetResult(await RunWithContextAsync(action).ConfigureAwait(false));
        }
        catch (Exception ex)
        {
            tcs.TrySetException(ex);
        }
    }

    private static async Task<T> RunWithContextAsync<T>(Func<Task<T>> action)
    {
        var previousContext = SynchronizationContext.Current;
        SynchronizationContext.SetSynchronizationContext(_context);
        try
        {
            return await action().ConfigureAwait(true);
        }
        finally
        {
            SynchronizationContext.SetSynchronizationContext(previousContext);
        }
    }
}
