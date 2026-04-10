using Godot;

namespace Spire2Mind.Bridge.Game.Util;

/// <summary>
/// Facade that delegates to <see cref="DynamicAccessor"/> and <see cref="TextExtractor"/>.
/// Existing callers can continue using ReflectionUtils without changes.
/// New code should reference DynamicAccessor or TextExtractor directly.
/// </summary>
internal static class ReflectionUtils
{
    // ── DynamicAccessor delegates ─────────────────────────────────

    public static object? GetMemberValue(object? instance, params string[] names)
        => DynamicAccessor.GetMemberValue(instance, names);

    public static T? GetMemberValue<T>(object? instance, params string[] names)
        => DynamicAccessor.GetMemberValue<T>(instance, names);

    public static object? InvokeMethod(object? instance, string methodName, params object?[] args)
        => DynamicAccessor.InvokeMethod(instance, methodName, args);

    public static bool TryInvokeMethod(object? instance, string methodName, params object?[] args)
        => DynamicAccessor.TryInvokeMethod(instance, methodName, args);

    public static bool IsVisible(object? instance)
        => DynamicAccessor.IsVisible(instance);

    public static bool IsEnabled(object? instance)
        => DynamicAccessor.IsEnabled(instance);

    public static bool IsAvailable(object? instance)
        => DynamicAccessor.IsAvailable(instance);

    public static IEnumerable<object> Enumerate(object? instance)
        => DynamicAccessor.Enumerate(instance);

    public static int? ToNullableInt(object? value)
        => DynamicAccessor.ToNullableInt(value);

    public static bool? ToNullableBool(object? value)
        => DynamicAccessor.ToNullableBool(value);

    // ── TextExtractor delegates ───────────────────────────────────

    public static string? LocalizedText(object? value)
        => TextExtractor.LocalizedText(value);

    public static string? ModelId(object? value)
        => TextExtractor.ModelId(value);

    public static IEnumerable<Node> Descendants(Node? node)
        => TextExtractor.Descendants(node);

    public static IEnumerable<Node> DescendantsByTypeName(Node? node, string typeName)
        => TextExtractor.DescendantsByTypeName(node, typeName);

    public static Node? FirstDescendantByTypeName(Node? node, string typeName)
        => TextExtractor.FirstDescendantByTypeName(node, typeName);
}
