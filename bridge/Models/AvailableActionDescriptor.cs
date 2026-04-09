namespace Spire2Mind.Bridge.Models;

internal sealed class AvailableActionDescriptor
{
    public string Action { get; init; } = string.Empty;

    public string Description { get; init; } = string.Empty;

    public IReadOnlyList<string> RequiredParameters { get; init; } = Array.Empty<string>();

    public IReadOnlyList<string> OptionalParameters { get; init; } = Array.Empty<string>();
}
