using System.Text.Json;
using Godot;

namespace Spire2Mind.Bridge.Game.Util;

internal static class GameVersionInfo
{
    private static readonly Lazy<(string? Version, string? Date)> Cached = new(ReadVersionInfo);

    public static string? Version => Cached.Value.Version;

    public static string? BuildDate => Cached.Value.Date;

    private static (string? Version, string? Date) ReadVersionInfo()
    {
        try
        {
            var gameDir = Path.GetDirectoryName(OS.GetExecutablePath());
            if (string.IsNullOrWhiteSpace(gameDir))
            {
                return (null, null);
            }

            var filePath = Path.Combine(gameDir, "release_info.json");
            if (!File.Exists(filePath))
            {
                return (null, null);
            }

            using var stream = File.OpenRead(filePath);
            using var document = JsonDocument.Parse(stream);
            var root = document.RootElement;
            var version = root.TryGetProperty("version", out var versionNode) ? versionNode.GetString() : null;
            var date = root.TryGetProperty("date", out var dateNode) ? dateNode.GetString() : null;
            return (version, date);
        }
        catch
        {
            return (null, null);
        }
    }
}
