using MegaCrit.Sts2.Core.Nodes;

namespace Spire2Mind.Bridge.Game.Actions;

/// <summary>
/// Executes game console commands (gold, heal, card, relic, fight, win, die, god, dump, etc.)
/// via the built-in DevConsole. Must run on the game thread.
/// </summary>
internal static class ConsoleCommandExecutor
{
    private static object? _cachedConsole;

    public static object Execute(string command)
    {
        var game = NGame.Instance;
        if (game == null)
        {
            return new { success = false, message = "Game not ready." };
        }

        var console = FindDevConsole(game);
        if (console == null)
        {
            return new { success = false, command, message = "DevConsole not found. Try pressing backtick (`) in-game first to activate the console." };
        }

        try
        {
            var result = DynamicAccessor.InvokeMethod(console, "ProcessCommand", command);
            return new { success = true, command, message = "Command executed." };
        }
        catch (Exception ex)
        {
            return new { success = false, command, message = ex.Message };
        }
    }

    private static object? FindDevConsole(NGame game)
    {
        // Return cached if still valid
        if (_cachedConsole != null)
        {
            return _cachedConsole;
        }

        // Search from scene root using FindChild (recursive)
        var root = game.GetTree()?.Root;
        if (root != null)
        {
            var node = root.FindChild("DevConsole", true, false);
            if (node != null)
            {
                _cachedConsole = node;
                return _cachedConsole;
            }
        }

        // Search via reflection on NGame
        _cachedConsole = DynamicAccessor.GetMemberValue(game, "DevConsole", "_devConsole");
        if (_cachedConsole != null)
        {
            return _cachedConsole;
        }

        // Full tree search by type name
        _cachedConsole = TextExtractor.Descendants(root ?? (Godot.Node)game)
            .FirstOrDefault(n => n.GetType().Name == "DevConsole");

        return _cachedConsole;
    }
}
