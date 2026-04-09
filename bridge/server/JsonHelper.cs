using System.Text;
using System.Text.Json;
using System.Text.Json.Serialization;

namespace Spire2Mind.Bridge.Http;

internal static class JsonHelper
{
    private static readonly JsonSerializerOptions Options = new()
    {
        PropertyNamingPolicy = JsonNamingPolicy.CamelCase,
        WriteIndented = true,
        Converters =
        {
            new JsonStringEnumConverter()
        }
    };

    public static byte[] SerializeToUtf8(object value)
    {
        return Encoding.UTF8.GetBytes(JsonSerializer.Serialize(value, Options));
    }

    public static T? Deserialize<T>(Stream stream)
    {
        return JsonSerializer.Deserialize<T>(stream, Options);
    }
}
