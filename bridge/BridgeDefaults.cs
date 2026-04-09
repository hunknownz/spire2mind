namespace Spire2Mind.Bridge;

internal static class BridgeDefaults
{
    public const int DefaultPort = 8080;

    public const int StateVersion = 1;

    public const int PortRetryCount = 20;

    public const int EventBufferCapacity = 256;

    public static readonly TimeSpan PortRetryDelay = TimeSpan.FromMilliseconds(250);

    public static readonly TimeSpan EventPollInterval = TimeSpan.FromMilliseconds(120);

    public static readonly TimeSpan EventKeepAliveInterval = TimeSpan.FromSeconds(15);
}
