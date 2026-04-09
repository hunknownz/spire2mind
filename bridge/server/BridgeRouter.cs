using System.Net;
using MegaCrit.Sts2.Core.Logging;
using Spire2Mind.Bridge.Game;
using Spire2Mind.Bridge.Models;

namespace Spire2Mind.Bridge.Http;

internal static class BridgeRouter
{
    public static async Task HandleAsync(HttpListenerContext context, CancellationToken cancellationToken)
    {
        var requestId = $"req_{Guid.NewGuid():N}";

        try
        {
            var path = context.Request.Url?.AbsolutePath.TrimEnd('/').ToLowerInvariant() ?? "/";
            var format = context.Request.QueryString["format"]?.ToLowerInvariant();

            switch ((context.Request.HttpMethod.ToUpperInvariant(), path))
            {
                case ("GET", ""):
                case ("GET", "/"):
                    await WriteErrorAsync(context, requestId, 404, "not_found", "Route not found.", retryable: false).ConfigureAwait(false);
                    return;

                case ("GET", "/health"):
                    await WriteSuccessAsync(
                        context,
                        requestId,
                        new HealthInfo
                        {
                            BridgeVersion = Entry.BridgeVersion,
                            GameVersion = GameVersionInfo.Version,
                            GameBuildDate = GameVersionInfo.BuildDate,
                            Port = BridgeRuntime.Port,
                            Ready = BridgeRuntime.IsReady
                        }).ConfigureAwait(false);
                    return;

                case ("GET", "/state"):
                {
                    var snapshot = BridgeRuntime.StateReadsEnabled
                        ? await GameThread.InvokeAsync(StateSnapshotBuilder.Build).ConfigureAwait(false)
                        : StateSnapshotBuilder.BuildBootstrap();
                    if (format == "markdown")
                    {
                        await WriteSuccessAsync(
                            context,
                            requestId,
                            new
                            {
                                format = "markdown",
                                markdown = StateMarkdownFormatter.FormatMarkdown(snapshot),
                                snapshot
                            }).ConfigureAwait(false);
                    }
                    else
                    {
                        await WriteSuccessAsync(context, requestId, snapshot).ConfigureAwait(false);
                    }

                    return;
                }

                case ("GET", "/actions/available"):
                {
                    var snapshot = BridgeRuntime.StateReadsEnabled
                        ? await GameThread.InvokeAsync(StateSnapshotBuilder.Build).ConfigureAwait(false)
                        : StateSnapshotBuilder.BuildBootstrap();
                    await WriteSuccessAsync(
                        context,
                        requestId,
                        new
                        {
                            screen = snapshot.Screen,
                            availableActions = snapshot.AvailableActions,
                            descriptors = AvailableActionCatalog.Describe(snapshot.AvailableActions)
                        }).ConfigureAwait(false);
                    return;
                }

                case ("POST", "/action"):
                {
                    if (!BridgeRuntime.StateReadsEnabled)
                    {
                        await WriteErrorAsync(
                            context,
                            requestId,
                            503,
                            "bridge_starting",
                            "Bridge is still waiting for the game to finish bootstrapping.",
                            retryable: true).ConfigureAwait(false);
                        return;
                    }

                    var request = JsonHelper.Deserialize<BridgeActionRequest>(context.Request.InputStream);
                    if (request == null)
                    {
                        await WriteErrorAsync(
                            context,
                            requestId,
                            400,
                            "invalid_request",
                            "Request body must be valid JSON.",
                            retryable: false).ConfigureAwait(false);
                        return;
                    }

                    var result = await GameThread.InvokeAsync(() => BridgeActionExecutor.ExecuteAsync(request)).ConfigureAwait(false);
                    await WriteSuccessAsync(context, requestId, result).ConfigureAwait(false);
                    return;
                }

                case ("GET", "/events/stream"):
                    if (BridgeRuntime.Events == null)
                    {
                        await WriteErrorAsync(
                            context,
                            requestId,
                            503,
                            "bridge_starting",
                            "Event stream is not ready yet.",
                            retryable: true).ConfigureAwait(false);
                        return;
                    }

                    await BridgeRuntime.Events.StreamAsync(context, requestId, cancellationToken).ConfigureAwait(false);
                    return;

                default:
                    await WriteErrorAsync(context, requestId, 404, "not_found", "Route not found.", retryable: false).ConfigureAwait(false);
                    return;
            }
        }
        catch (BridgeApiException ex)
        {
            Log.Warn($"[{Entry.ModId}] API error {ex.Code} on {context.Request.HttpMethod} {context.Request.Url?.AbsolutePath}: {ex}");
            await WriteErrorAsync(
                context,
                requestId,
                ex.StatusCode,
                ex.Code,
                ex.Message,
                ex.Retryable,
                ex.Details).ConfigureAwait(false);
        }
        catch (Exception ex)
        {
            Log.Error($"[{Entry.ModId}] Unhandled error on {context.Request.HttpMethod} {context.Request.Url?.AbsolutePath}: {ex}");
            await WriteErrorAsync(
                context,
                requestId,
                500,
                "internal_error",
                ex.Message,
                retryable: false).ConfigureAwait(false);
        }
        finally
        {
            if (context.Response.OutputStream.CanWrite)
            {
                context.Response.OutputStream.Close();
            }
        }
    }

    private static Task WriteSuccessAsync(HttpListenerContext context, string requestId, object data)
    {
        var payload = new
        {
            ok = true,
            request_id = requestId,
            data
        };

        return WriteJsonAsync(context, 200, payload);
    }

    private static Task WriteErrorAsync(
        HttpListenerContext context,
        string requestId,
        int statusCode,
        string code,
        string message,
        bool retryable,
        object? details = null)
    {
        var payload = new
        {
            ok = false,
            request_id = requestId,
            error = new
            {
                code,
                message,
                retryable,
                details
            }
        };

        return WriteJsonAsync(context, statusCode, payload);
    }

    private static async Task WriteJsonAsync(HttpListenerContext context, int statusCode, object payload)
    {
        var buffer = JsonHelper.SerializeToUtf8(payload);

        context.Response.StatusCode = statusCode;
        context.Response.ContentType = "application/json; charset=utf-8";
        context.Response.ContentLength64 = buffer.Length;
        await context.Response.OutputStream.WriteAsync(buffer, 0, buffer.Length).ConfigureAwait(false);
    }
}
