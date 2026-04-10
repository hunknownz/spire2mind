using System.Collections;
using System.Reflection;
using Godot;

namespace Spire2Mind.Bridge.Game.Util;

internal static class ReflectionUtils
{
    private const BindingFlags AnyInstance = BindingFlags.Instance | BindingFlags.Public | BindingFlags.NonPublic;

    public static object? GetMemberValue(object? instance, params string[] names)
    {
        if (instance == null)
        {
            return null;
        }

        var type = instance.GetType();
        foreach (var name in names)
        {
            var property = type.GetProperty(name, AnyInstance);
            if (property != null)
            {
                return property.GetValue(instance);
            }

            var field = type.GetField(name, AnyInstance);
            if (field != null)
            {
                return field.GetValue(instance);
            }
        }

        return null;
    }

    public static T? GetMemberValue<T>(object? instance, params string[] names)
    {
        var value = GetMemberValue(instance, names);
        if (value == null)
        {
            return default;
        }

        if (value is T typed)
        {
            return typed;
        }

        try
        {
            return (T?)Convert.ChangeType(value, typeof(T));
        }
        catch
        {
            return default;
        }
    }

    public static object? InvokeMethod(object? instance, string methodName, params object?[] args)
    {
        if (instance == null)
        {
            return null;
        }

        var methods = instance.GetType().GetMethods(AnyInstance)
            .Where(method => method.Name == methodName && method.GetParameters().Length == args.Length);

        foreach (var method in methods)
        {
            var parameters = method.GetParameters();
            var compatible = true;
            for (var i = 0; i < parameters.Length; i++)
            {
                if (args[i] == null)
                {
                    continue;
                }

                if (!parameters[i].ParameterType.IsInstanceOfType(args[i]) &&
                    !CanConvert(args[i]!, parameters[i].ParameterType))
                {
                    compatible = false;
                    break;
                }
            }

            if (!compatible)
            {
                continue;
            }

            var converted = new object?[args.Length];
            for (var i = 0; i < args.Length; i++)
            {
                converted[i] = ConvertArgument(args[i], parameters[i].ParameterType);
            }

            return method.Invoke(instance, converted);
        }

        return null;
    }

    public static bool TryInvokeMethod(object? instance, string methodName, params object?[] args)
    {
        if (instance == null)
        {
            return false;
        }

        var methods = instance.GetType().GetMethods(AnyInstance)
            .Where(method => method.Name == methodName && method.GetParameters().Length == args.Length);

        foreach (var method in methods)
        {
            var parameters = method.GetParameters();
            var compatible = true;
            for (var i = 0; i < parameters.Length; i++)
            {
                if (args[i] == null)
                {
                    continue;
                }

                if (!parameters[i].ParameterType.IsInstanceOfType(args[i]) &&
                    !CanConvert(args[i]!, parameters[i].ParameterType))
                {
                    compatible = false;
                    break;
                }
            }

            if (!compatible)
            {
                continue;
            }

            var converted = new object?[args.Length];
            for (var i = 0; i < args.Length; i++)
            {
                converted[i] = ConvertArgument(args[i], parameters[i].ParameterType);
            }

            method.Invoke(instance, converted);
            return true;
        }

        return false;
    }

    public static bool IsVisible(object? instance)
    {
        if (instance == null)
        {
            return false;
        }

        if (instance is CanvasItem canvasItem)
        {
            return canvasItem.Visible && canvasItem.IsVisibleInTree();
        }

        return true;
    }

    public static bool IsEnabled(object? instance)
    {
        if (instance == null)
        {
            return false;
        }

        var isEnabled = GetMemberValue<bool?>(instance, "IsEnabled", "_isEnabled", "Enabled");
        if (isEnabled != null)
        {
            return isEnabled == true;
        }

        var disabled = GetMemberValue<bool?>(instance, "Disabled", "IsDisabled");
        return disabled != true;
    }

    public static bool IsAvailable(object? instance)
    {
        return IsVisible(instance) && IsEnabled(instance);
    }

    public static IEnumerable<object> Enumerate(object? instance)
    {
        if (instance == null || instance is string)
        {
            yield break;
        }

        if (instance is IEnumerable enumerable)
        {
            foreach (var item in enumerable)
            {
                if (item != null)
                {
                    yield return item;
                }
            }
        }
    }

    public static IEnumerable<Node> Descendants(Node? node)
    {
        if (node == null)
        {
            yield break;
        }

        foreach (Node child in node.GetChildren())
        {
            yield return child;

            foreach (var descendant in Descendants(child))
            {
                yield return descendant;
            }
        }
    }

    public static IEnumerable<Node> DescendantsByTypeName(Node? node, string typeName)
    {
        return Descendants(node).Where(candidate => candidate.GetType().Name == typeName);
    }

    public static Node? FirstDescendantByTypeName(Node? node, string typeName)
    {
        return DescendantsByTypeName(node, typeName).FirstOrDefault();
    }

    public static string? LocalizedText(object? value)
    {
        if (value == null)
        {
            return null;
        }

        if (value is string text)
        {
            return string.IsNullOrWhiteSpace(text) ? null : text;
        }

        if (value.GetType().Name == "LocString")
        {
            try
            {
                var formatted = InvokeMethod(value, "GetFormattedText") as string;
                if (!string.IsNullOrWhiteSpace(formatted))
                {
                    return formatted;
                }
            }
            catch
            {
                // Some localized event strings reference gameplay selectors that are only valid
                // inside the game's own formatting pipeline. Fall back to raw text instead.
            }

            try
            {
                var raw = InvokeMethod(value, "GetRawText") as string;
                if (!string.IsNullOrWhiteSpace(raw))
                {
                    return raw;
                }
            }
            catch
            {
            }
        }

        var directText = GetMemberValue<string>(value, "Name", "Title", "Description", "Text", "Message");
        if (!string.IsNullOrWhiteSpace(directText))
        {
            return directText;
        }

        foreach (var memberName in new[] { "Title", "Description", "Name", "Id" })
        {
            var nested = GetMemberValue(value, memberName);
            var nestedText = LocalizedText(nested);
            if (!string.IsNullOrWhiteSpace(nestedText))
            {
                return nestedText;
            }
        }

        var entry = GetMemberValue<string>(value, "Entry", "TextKey", "Hotkey", "OptionId");
        if (!string.IsNullOrWhiteSpace(entry))
        {
            return entry;
        }

        var fallback = value.ToString();
        return LooksLikeTypeName(fallback) ? null : fallback;
    }

    private static bool LooksLikeTypeName(string? value)
    {
        if (string.IsNullOrWhiteSpace(value))
        {
            return false;
        }

        return value.Contains('.') && !value.Any(char.IsWhiteSpace);
    }

    public static string? ModelId(object? value)
    {
        if (value == null)
        {
            return null;
        }

        if (value is string text)
        {
            return text;
        }

        var entry = GetMemberValue<string>(value, "Entry");
        if (!string.IsNullOrWhiteSpace(entry))
        {
            return entry;
        }

        var id = GetMemberValue(value, "Id");
        return ModelId(id);
    }

    public static int? ToNullableInt(object? value)
    {
        if (value == null)
        {
            return null;
        }

        try
        {
            return Convert.ToInt32(value);
        }
        catch
        {
            return null;
        }
    }

    public static bool? ToNullableBool(object? value)
    {
        if (value == null)
        {
            return null;
        }

        try
        {
            return Convert.ToBoolean(value);
        }
        catch
        {
            return null;
        }
    }

    private static bool CanConvert(object value, Type targetType)
    {
        if (targetType.IsEnum)
        {
            return value is string || value.GetType().IsPrimitive;
        }

        return value is IConvertible;
    }

    private static object? ConvertArgument(object? value, Type targetType)
    {
        if (value == null)
        {
            return null;
        }

        if (targetType.IsInstanceOfType(value))
        {
            return value;
        }

        if (targetType.IsEnum && value is string enumName)
        {
            return Enum.Parse(targetType, enumName);
        }

        return Convert.ChangeType(value, targetType);
    }
}
