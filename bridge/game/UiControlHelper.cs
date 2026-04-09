namespace Spire2Mind.Bridge.Game;

internal static class UiControlHelper
{
    public static bool IsAvailable(object? control)
    {
        return ReflectionUtils.IsAvailable(control);
    }

    public static bool HasAvailableControl(object owner, params string[] memberNames)
    {
        return IsAvailable(ReflectionUtils.GetMemberValue(owner, memberNames));
    }
}
