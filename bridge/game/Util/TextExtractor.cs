using Godot;

namespace Spire2Mind.Bridge.Game.Util;

/// <summary>
/// Extracts display text and model identifiers from game objects.
/// Handles LocString resolution, fallback chains, and model ID extraction.
/// </summary>
internal static class TextExtractor
{
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
            var locResult = TryExtractLocString(value);
            if (locResult != null)
            {
                return locResult;
            }
        }

        var directText = DynamicAccessor.GetMemberValue<string>(value, "Name", "Title", "Description", "Text", "Message");
        if (!string.IsNullOrWhiteSpace(directText))
        {
            return directText;
        }

        foreach (var memberName in new[] { "Title", "Description", "Name", "Id" })
        {
            var nested = DynamicAccessor.GetMemberValue(value, memberName);
            var nestedText = LocalizedText(nested);
            if (!string.IsNullOrWhiteSpace(nestedText))
            {
                return nestedText;
            }
        }

        var entry = DynamicAccessor.GetMemberValue<string>(value, "Entry", "TextKey", "Hotkey", "OptionId");
        if (!string.IsNullOrWhiteSpace(entry))
        {
            return entry;
        }

        var fallback = value.ToString();
        return LooksLikeTypeName(fallback) ? null : fallback;
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

        var entry = DynamicAccessor.GetMemberValue<string>(value, "Entry");
        if (!string.IsNullOrWhiteSpace(entry))
        {
            return entry;
        }

        var id = DynamicAccessor.GetMemberValue(value, "Id");
        return ModelId(id);
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

    private static string? TryExtractLocString(object value)
    {
        try
        {
            var formatted = DynamicAccessor.InvokeMethod(value, "GetFormattedText") as string;
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
            var raw = DynamicAccessor.InvokeMethod(value, "GetRawText") as string;
            if (!string.IsNullOrWhiteSpace(raw))
            {
                return raw;
            }
        }
        catch
        {
        }

        return null;
    }

    private static bool LooksLikeTypeName(string? value)
    {
        if (string.IsNullOrWhiteSpace(value))
        {
            return false;
        }

        return value.Contains('.') && !value.Any(char.IsWhiteSpace);
    }
}
