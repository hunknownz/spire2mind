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

        _listener = StartListenerWithRetry($"http://{Host}:{Port}/");
        _acceptLoop = Task.Run(() => AcceptLoopAsync(_listener, _cts.Token), _cts.Token);
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
