using System.Text.Json.Serialization;

namespace Spire2Mind.Bridge.Models;

internal sealed class BridgeActionRequest
{
    [JsonPropertyName("action")]
    public string? Action { get; init; }

    [JsonPropertyName("card_index")]
    public int? CardIndex { get; init; }

    [JsonPropertyName("target_index")]
    public int? TargetIndex { get; init; }

    [JsonPropertyName("option_index")]
    public int? OptionIndex { get; init; }
}
