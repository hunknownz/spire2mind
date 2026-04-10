using System.Collections;
using System.Reflection;
using Godot;

namespace Spire2Mind.Bridge.Game.Util;

/// <summary>
/// Runtime reflection utilities for accessing game object members dynamically.
/// Used when compile-time types are unavailable (untyped UI nodes, private members
/// without Publicizer, multi-name probing for version compatibility).
/// </summary>
internal static class DynamicAccessor
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
            if (!AreArgsCompatible(parameters, args))
            {
                continue;
            }

            var converted = ConvertArgs(parameters, args);
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
            if (!AreArgsCompatible(parameters, args))
            {
                continue;
            }

            var converted = ConvertArgs(parameters, args);
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

    private static bool AreArgsCompatible(ParameterInfo[] parameters, object?[] args)
    {
        for (var i = 0; i < parameters.Length; i++)
        {
            if (args[i] == null)
            {
                continue;
            }

            if (!parameters[i].ParameterType.IsInstanceOfType(args[i]) &&
                !CanConvert(args[i]!, parameters[i].ParameterType))
            {
                return false;
            }
        }

        return true;
    }

    private static object?[] ConvertArgs(ParameterInfo[] parameters, object?[] args)
    {
        var converted = new object?[args.Length];
        for (var i = 0; i < args.Length; i++)
        {
            converted[i] = ConvertArgument(args[i], parameters[i].ParameterType);
        }

        return converted;
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
