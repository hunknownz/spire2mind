using MegaCrit.Sts2.Core.DevConsole;
using MegaCrit.Sts2.Core.Nodes;

namespace Spire2Mind.Bridge.Game.Actions;

/// <summary>
/// Executes game console commands (gold, heal, card, relic, fight, win, die, god, dump, etc.)
/// via the built-in DevConsole. Must run on the game thread.
/// </summary>
internal static class ConsoleCommandExecutor
{
    public static object Execute(string command)
    {
        var game = NGame.Instance;
        if (game == null)
        {
            return new { success = false, message = "Game not ready." };
        }

        // DevConsole is a Godot node in the scene tree. Search common paths.
        var console = FindDevConsole(game);
        if (console != null)
        {
            try
            {
                console.ProcessCommand(command);
                return new { success = true, command, message = "Command executed." };
            }
            catch (Exception ex)
            {
                return new { success = false, command, message = ex.Message };
            }
        }

        // Fallback: try invoking the command processor via reflection
        try
        {
            var result = DynamicAccessor.InvokeMethod(
                DynamicAccessor.GetMemberValue(game, "DevConsole", "_devConsole", "Console"),
                "ProcessCommand", command);
            if (result != null)
            {
                return new { success = true, command, message = "Command executed (fallback)." };
            }
        }
        catch
        {
        }

        return new { success = false, command, message = "DevConsole not available. The game may need to be launched with the debug console enabled (backtick key)." };
    }

    private static DevConsole? FindDevConsole(NGame game)
    {
        // Try direct paths
        var console = game.GetNodeOrNull<DevConsole>("%DevConsole")
            ?? game.GetNodeOrNull<DevConsole>("DevConsole")
            ?? game.GetNodeOrNull<DevConsole>("Ui/DevConsole")
            ?? game.GetNodeOrNull<DevConsole>("/root/NGame/DevConsole");

        if (console != null)
        {
            return console;
        }

        // Search entire tree
        return TextExtractor.Descendants(game)
            .OfType<DevConsole>()
            .FirstOrDefault();
    }
}
