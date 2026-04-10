using MegaCrit.Sts2.Core.Context;
using MegaCrit.Sts2.Core.Entities.Players;
using MegaCrit.Sts2.Core.Nodes;

namespace Spire2Mind.Bridge.Game.Actions;

/// <summary>
/// Executes game console commands (gold, heal, card, relic, fight, win, die, god, dump, etc.)
/// via the built-in DevConsole. Must run on the game thread.
///
/// Architecture: NDevConsole (Godot Node at /root/Game/DevConsole/ConsoleScreen)
/// holds a DevConsole instance (plain C# object) that has ProcessCommand(string).
/// We find NDevConsole → extract its DevConsole field → call ProcessCommand.
/// </summary>
internal static class ConsoleCommandExecutor
{
    private static object? _cachedDevConsole;

    public static object Execute(string command)
    {
        var game = NGame.Instance;
        if (game == null)
        {
            return new { success = false, message = "Game not ready." };
        }

        var devConsole = FindDevConsoleInstance(game);
        if (devConsole == null)
        {
            return new { success = false, command, message = "DevConsole not found. Press backtick (`) in-game to activate the console first." };
        }

        try
        {
            // DevConsole.ProcessCommand(Player?, String, String[]) or ProcessCommand(String)
            // Try the string overload first
            var result = DynamicAccessor.InvokeMethod(devConsole, "ProcessCommand", command);

            // If that didn't work (returned null and method not found), try with player
            if (result == null)
            {
                var player = LocalContext.GetMe(MegaCrit.Sts2.Core.Runs.RunManager.Instance?.DebugOnlyGetState()) as Player;
                var parts = command.Split(' ', 2);
                var cmdName = parts[0];
                var args = parts.Length > 1 ? parts[1].Split(' ') : Array.Empty<string>();
                DynamicAccessor.InvokeMethod(devConsole, "ProcessCommandInternal", player, new[] { cmdName }.Concat(args).ToArray());
            }

            return new { success = true, command, message = "Command executed." };
        }
        catch (Exception ex)
        {
            return new { success = false, command, message = ex.Message };
        }
    }

    private static object? FindDevConsoleInstance(NGame game)
    {
        if (_cachedDevConsole != null)
        {
            return _cachedDevConsole;
        }

        // Find NDevConsole node at known path
        var consoleScreen = game.GetTree()?.Root?.FindChild("ConsoleScreen", true, false);
        if (consoleScreen == null)
        {
            return null;
        }

        // Extract the DevConsole field from NDevConsole
        // NDevConsole holds a DevConsole instance internally
        _cachedDevConsole = DynamicAccessor.GetMemberValue(consoleScreen,
            "_console", "Console", "_devConsole", "DevConsole", "console");

        if (_cachedDevConsole != null)
        {
            return _cachedDevConsole;
        }

        // Fallback: search all fields for a DevConsole-typed object
        var fields = consoleScreen.GetType().GetFields(
            System.Reflection.BindingFlags.Instance |
            System.Reflection.BindingFlags.Public |
            System.Reflection.BindingFlags.NonPublic);

        foreach (var field in fields)
        {
            if (field.FieldType.Name == "DevConsole")
            {
                _cachedDevConsole = field.GetValue(consoleScreen);
                if (_cachedDevConsole != null)
                {
                    return _cachedDevConsole;
                }
            }
        }

        return null;
    }
}
