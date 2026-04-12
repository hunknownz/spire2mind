using System.Net;

namespace Spire2Mind.Bridge.Http;

internal sealed class HttpServer : IDisposable
{
    private readonly Func<HttpListenerContext, CancellationToken, Task> _handler;

    private readonly CancellationTokenSource _cts = new();

    private HttpListener? _listener;

    private Task? _acceptLoop;

    public HttpServer(Func<HttpListenerContext, CancellationToken, Task> handler)
    {
        _handler = handler;
        Port = ResolvePort();
        Host = ResolveHost();
    }

    public int Port { get; }

    public string Host { get; }

    public bool IsRunning => _listener?.IsListening == true;

    public void Start()
    {
        if (_listener != null)
        {
            return;
        }

        var envHost = Environment.GetEnvironmentVariable("STS2_API_HOST");
        var envPort = Environment.GetEnvironmentVariable("STS2_API_PORT");
        Diag($"Start() called. STS2_API_HOST={ReprEnv(envHost)} STS2_API_PORT={ReprEnv(envPort)} ResolvedHost={Host} ResolvedPort={Port}");

        var candidates = BuildPrefixCandidates(Host, Port);
        Diag($"Candidates ({candidates.Count}): {string.Join(", ", candidates)}");

        Exception? lastException = null;
        foreach (var prefix in candidates)
        {
            Diag($"Trying {prefix}");
            try
            {
                _listener = StartListenerWithRetry(prefix);
                Diag($"Bound {prefix}");
                ActivePrefix = prefix;
                break;
            }
            catch (Exception ex)
            {
                lastException = ex;
                var he = ex as HttpListenerException;
                Diag($"Failed {prefix}: {ex.GetType().Name}: {ex.Message}{(he != null ? $" (ErrorCode={he.ErrorCode})" : "")}");
            }
        }

        if (_listener == null)
        {
            var last = lastException?.Message ?? "(no exception captured)";
            throw new InvalidOperationException(
                $"All {candidates.Count} listener prefix(es) failed. Last error: {last}",
                lastException);
        }

        _acceptLoop = Task.Run(() => AcceptLoopAsync(_listener, _cts.Token), _cts.Token);
    }

    public string? ActivePrefix { get; private set; }

    private static readonly string DiagLogPath = Path.Combine(
        Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData),
        "SlayTheSpire2",
        "spire2mind-bridge-diag.log");

    private static readonly object DiagLock = new();

    private static void Diag(string message)
    {
        try
        {
            var line = $"[{DateTime.Now:yyyy-MM-dd HH:mm:ss.fff}] {message}{Environment.NewLine}";
            lock (DiagLock)
            {
                Directory.CreateDirectory(Path.GetDirectoryName(DiagLogPath)!);
                File.AppendAllText(DiagLogPath, line);
            }
        }
        catch
        {
            // Best-effort only.
        }
    }

    private static string ReprEnv(string? value)
    {
        if (value == null)
        {
            return "<null>";
        }

        return $"\"{value}\"";
    }

    private static List<string> BuildPrefixCandidates(string host, int port)
    {
        var list = new List<string> { $"http://{host}:{port}/" };

        if (!string.Equals(host, "+", StringComparison.Ordinal)
            && !string.Equals(host, "127.0.0.1", StringComparison.Ordinal))
        {
            list.Add($"http://+:{port}/");
        }

        if (!string.Equals(host, "127.0.0.1", StringComparison.Ordinal))
        {
            list.Add($"http://127.0.0.1:{port}/");
        }

        return list;
    }

    public void Dispose()
    {
        _cts.Cancel();

        try
        {
            _listener?.Close();
            _acceptLoop?.Wait(TimeSpan.FromSeconds(2));
        }
        catch
        {
            // Best-effort shutdown only.
        }
        finally
        {
            _listener = null;
        }
    }

    private async Task AcceptLoopAsync(HttpListener listener, CancellationToken token)
    {
        while (!token.IsCancellationRequested && listener.IsListening)
        {
            HttpListenerContext? context = null;
            try
            {
                context = await listener.GetContextAsync().ConfigureAwait(false);
            }
            catch (HttpListenerException)
            {
                break;
            }
            catch (ObjectDisposedException)
            {
                break;
            }

            _ = Task.Run(
                async () =>
                {
                    try
                    {
                        await _handler(context, token).ConfigureAwait(false);
                    }
                    catch
                    {
                        if (context.Response.OutputStream.CanWrite)
                        {
                            context.Response.StatusCode = 500;
                            context.Response.Close();
                        }
                    }
                },
                token);
        }
    }

    private static int ResolvePort()
    {
        var rawValue = ResolveMultiScope("STS2_API_PORT");
        return int.TryParse(rawValue, out var port) ? port : BridgeDefaults.DefaultPort;
    }

    private static string ResolveHost()
    {
        var rawValue = ResolveMultiScope("STS2_API_HOST");
        if (string.IsNullOrWhiteSpace(rawValue))
        {
            return "127.0.0.1";
        }

        var trimmed = rawValue.Trim();
        if (string.Equals(trimmed, "0.0.0.0", StringComparison.Ordinal))
        {
            return "+";
        }

        return trimmed;
    }

    /// <summary>
    /// Reads an env var from Process, then User, then Machine scope. The registry-backed
    /// User/Machine reads bypass the game process's inherited env block, which matters
    /// when Steam (the usual game launcher) was started before the variable was set.
    /// </summary>
    private static string? ResolveMultiScope(string name)
    {
        var process = Environment.GetEnvironmentVariable(name);
        if (!string.IsNullOrWhiteSpace(process))
        {
            return process;
        }

        try
        {
            var user = Environment.GetEnvironmentVariable(name, EnvironmentVariableTarget.User);
            if (!string.IsNullOrWhiteSpace(user))
            {
                return user;
            }
        }
        catch
        {
            // registry access denied — fall through
        }

        try
        {
            return Environment.GetEnvironmentVariable(name, EnvironmentVariableTarget.Machine);
        }
        catch
        {
            return null;
        }
    }

    private static HttpListener StartListenerWithRetry(string prefix)
    {
        for (var attempt = 1; ; attempt++)
        {
            var listener = new HttpListener();
            listener.Prefixes.Add(prefix);

            try
            {
                listener.Start();
                return listener;
            }
            catch (HttpListenerException ex) when (IsPrefixConflict(ex) && attempt < BridgeDefaults.PortRetryCount)
            {
                listener.Close();
                Thread.Sleep(BridgeDefaults.PortRetryDelay);
            }
        }
    }

    private static bool IsPrefixConflict(HttpListenerException ex)
    {
        return ex.ErrorCode is 183 or 32;
    }
}
