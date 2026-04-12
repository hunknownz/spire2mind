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

        var candidates = BuildPrefixCandidates(Host, Port);
        Exception? lastException = null;
        foreach (var prefix in candidates)
        {
            Console.Error.WriteLine($"[Spire2Mind.Bridge] Trying prefix: {prefix}");
            try
            {
                _listener = StartListenerWithRetry(prefix);
                Console.Error.WriteLine($"[Spire2Mind.Bridge] Bound prefix: {prefix}");
                ActivePrefix = prefix;
                break;
            }
            catch (Exception ex)
            {
                lastException = ex;
                Console.Error.WriteLine($"[Spire2Mind.Bridge] Prefix {prefix} failed: {ex.GetType().Name}: {ex.Message}");
            }
        }

        if (_listener == null)
        {
            throw new InvalidOperationException(
                $"All listener prefixes failed (tried {candidates.Count}). See earlier [Spire2Mind.Bridge] log entries.",
                lastException);
        }

        _acceptLoop = Task.Run(() => AcceptLoopAsync(_listener, _cts.Token), _cts.Token);
    }

    public string? ActivePrefix { get; private set; }

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
        var rawValue = Environment.GetEnvironmentVariable("STS2_API_PORT");
        return int.TryParse(rawValue, out var port) ? port : BridgeDefaults.DefaultPort;
    }

    private static string ResolveHost()
    {
        var rawValue = Environment.GetEnvironmentVariable("STS2_API_HOST");
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
