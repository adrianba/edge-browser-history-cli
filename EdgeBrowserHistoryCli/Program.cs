using System.Globalization;
using System.Text.Json;
using System.Text.Json.Serialization;
using System.Data.SQLite;

using var cts = new CancellationTokenSource(TimeSpan.FromSeconds(30));
return await EdgeHistoryCli.RunAsync(args, cts.Token);

internal static class EdgeHistoryCli
{
    private static readonly JsonSerializerOptions JsonOptions = new()
    {
        PropertyNamingPolicy = JsonNamingPolicy.CamelCase,
        WriteIndented = true,
        DefaultIgnoreCondition = JsonIgnoreCondition.WhenWritingNull,
    };

    public static async Task<int> RunAsync(string[] args, CancellationToken cancellationToken = default)
    {
        if (args.Length == 0 || args.Contains("--help", StringComparer.OrdinalIgnoreCase))
        {
            PrintHelp();
            return 0;
        }

        try
        {
            var parsed = CliArguments.Parse(args);
            var userDataDir = ResolveUserDataDir(parsed.UserDataDir);

            if (parsed.ListProfiles)
            {
                var profiles = EdgeHistoryService.ListProfiles(userDataDir);
                WriteJson(new { profiles });
                return 0;
            }

            if (parsed.HistoryRequest is null)
            {
                throw new EdgeHistoryException("No command specified. Use --profiles or --history.");
            }

            var history = await EdgeHistoryService.GetHistoryAsync(userDataDir, parsed.HistoryRequest, cancellationToken);
            WriteJson(new
            {
                profile = parsed.HistoryRequest.Profile,
                date = parsed.HistoryRequest.Date,
                startTime = parsed.HistoryRequest.StartTime,
                endTime = parsed.HistoryRequest.EndTime,
                entries = history,
            });
            return 0;
        }
        catch (EdgeHistoryException ex)
        {
            WriteJson(new { error = ex.Message });
            return 1;
        }
        catch (OperationCanceledException)
        {
            WriteJson(new { error = "Operation timed out." });
            return 1;
        }
    }

    private static string ResolveUserDataDir(string? overrideDir)
    {
        string resolved;

        if (!string.IsNullOrWhiteSpace(overrideDir))
        {
            resolved = overrideDir;
        }
        else
        {
            var envOverride = Environment.GetEnvironmentVariable("EDGE_USER_DATA_DIR");
            if (!string.IsNullOrWhiteSpace(envOverride))
            {
                resolved = envOverride;
            }
            else
            {
                var localAppData = Environment.GetEnvironmentVariable("LOCALAPPDATA");
                if (string.IsNullOrWhiteSpace(localAppData))
                {
                    throw new EdgeHistoryException("LOCALAPPDATA is not set; cannot locate Edge data. Set EDGE_USER_DATA_DIR to override.");
                }

                resolved = Path.Combine(localAppData, "Microsoft", "Edge", "User Data");
            }
        }

        if (!Directory.Exists(resolved))
        {
            throw new EdgeHistoryException($"Edge User Data directory not found: {resolved}");
        }

        return resolved;
    }

    private static void PrintHelp()
    {
        Console.WriteLine("Edge Browser History CLI");
        Console.WriteLine();
        Console.WriteLine("Usage:");
        Console.WriteLine("  edge-browser-history-cli --help");
        Console.WriteLine("  edge-browser-history-cli --profiles [--user-data-dir <path>]");
        Console.WriteLine("  edge-browser-history-cli --history --profile <name-or-directory> --date <yyyy-MM-dd> [--start-time <HH:mm[:ss]>] [--end-time <HH:mm[:ss]>] [--user-data-dir <path>]");
        Console.WriteLine();
        Console.WriteLine("Options:");
        Console.WriteLine("  --help                 Show this help text.");
        Console.WriteLine("  --profiles             List available Edge browser profiles.");
        Console.WriteLine("  --history              Return browsing history for a given profile and date.");
        Console.WriteLine("  --profile              Profile name or directory id (e.g. Default, Profile 1).");
        Console.WriteLine("  --date                 Local date in yyyy-MM-dd format.");
        Console.WriteLine("  --start-time           Optional local start time in HH:mm or HH:mm:ss.");
        Console.WriteLine("  --end-time             Optional local end time in HH:mm or HH:mm:ss (exclusive). Must be after start-time.");
        Console.WriteLine("  --user-data-dir        Optional override for Edge User Data directory.");
    }

    private static void WriteJson(object payload)
    {
        Console.WriteLine(JsonSerializer.Serialize(payload, JsonOptions));
    }
}

public sealed class EdgeHistoryException(string message) : Exception(message);

public sealed record CliArguments(bool ListProfiles, HistoryRequest? HistoryRequest, string? UserDataDir)
{
    public static CliArguments Parse(string[] args)
    {
        var values = new Dictionary<string, string?>(StringComparer.OrdinalIgnoreCase);
        var flags = new HashSet<string>(StringComparer.OrdinalIgnoreCase);

        for (var i = 0; i < args.Length; i++)
        {
            var arg = args[i];
            if (!arg.StartsWith("--", StringComparison.Ordinal))
            {
                throw new EdgeHistoryException($"Unexpected argument: {arg}");
            }

            if (arg is "--profiles" or "--history")
            {
                flags.Add(arg);
                continue;
            }

            if (i + 1 >= args.Length || args[i + 1].StartsWith("--", StringComparison.Ordinal))
            {
                throw new EdgeHistoryException($"Missing value for {arg}.");
            }

            values[arg] = args[++i];
        }

        var hasProfiles = flags.Contains("--profiles");
        var hasHistory = flags.Contains("--history");

        if (hasProfiles == hasHistory)
        {
            throw new EdgeHistoryException("Specify exactly one of --profiles or --history.");
        }

        values.TryGetValue("--user-data-dir", out var userDataDir);

        if (hasProfiles)
        {
            return new CliArguments(ListProfiles: true, HistoryRequest: null, UserDataDir: userDataDir);
        }

        if (!values.TryGetValue("--profile", out var profile) || string.IsNullOrWhiteSpace(profile))
        {
            throw new EdgeHistoryException("--history requires --profile.");
        }

        if (!values.TryGetValue("--date", out var date) || string.IsNullOrWhiteSpace(date))
        {
            throw new EdgeHistoryException("--history requires --date in yyyy-MM-dd format.");
        }

        values.TryGetValue("--start-time", out var startTime);
        values.TryGetValue("--end-time", out var endTime);

        return new CliArguments(
            ListProfiles: false,
            HistoryRequest: new HistoryRequest(profile, date, startTime, endTime),
            UserDataDir: userDataDir);
    }
}

public sealed record HistoryRequest(string Profile, string Date, string? StartTime, string? EndTime);

public sealed record ProfileInfo(string Name, string Directory, bool IsDefault);

public sealed record HistoryEntry(
    string VisitTime,
    string Url,
    string Title,
    int VisitCount,
    int TypedCount,
    string Transition,
    long UrlId,
    long VisitId);

public static class EdgeHistoryService
{
    private static readonly HashSet<string> NonBrowsingProfiles = new(StringComparer.OrdinalIgnoreCase)
    {
        "Guest Profile",
        "System Profile",
    };

    private static readonly Dictionary<int, string> TransitionMap = new()
    {
        [0] = "link",
        [1] = "typed",
        [2] = "auto_bookmark",
        [3] = "auto_subframe",
        [4] = "manual_subframe",
        [5] = "generated",
        [6] = "auto_toplevel",
        [7] = "form_submit",
        [8] = "reload",
        [9] = "keyword",
        [10] = "keyword_generated",
    };

    public static IReadOnlyList<ProfileInfo> ListProfiles(string userDataDir)
    {
        var baseDir = new DirectoryInfo(userDataDir);
        if (!baseDir.Exists)
        {
            throw new EdgeHistoryException($"Edge User Data directory not found: {userDataDir}");
        }

        var infoCache = LoadInfoCache(Path.Combine(userDataDir, "Local State"));
        var profiles = new List<ProfileInfo>();
        var seenNames = new Dictionary<string, int>(StringComparer.Ordinal);

        foreach (var kvp in infoCache.OrderBy(item => item.Key, StringComparer.Ordinal))
        {
            var directory = kvp.Key;
            if (NonBrowsingProfiles.Contains(directory) || !IsSafeProfileDirectory(directory))
            {
                continue;
            }

            var historyPath = Path.Combine(userDataDir, directory, "History");
            if (!File.Exists(historyPath))
            {
                continue;
            }

            var friendlyName = string.IsNullOrWhiteSpace(kvp.Value) ? directory : kvp.Value!;
            seenNames[friendlyName] = seenNames.TryGetValue(friendlyName, out var count) ? count + 1 : 1;
            profiles.Add(new ProfileInfo(friendlyName, directory, string.Equals(directory, "Default", StringComparison.Ordinal)));
        }

        if (profiles.Count == 0)
        {
            foreach (var profileDir in baseDir.EnumerateDirectories().OrderBy(d => d.Name, StringComparer.Ordinal))
            {
                if (NonBrowsingProfiles.Contains(profileDir.Name) || !IsSafeProfileDirectory(profileDir.Name))
                {
                    continue;
                }

                if (!File.Exists(Path.Combine(profileDir.FullName, "History")))
                {
                    continue;
                }

                profiles.Add(new ProfileInfo(profileDir.Name, profileDir.Name, string.Equals(profileDir.Name, "Default", StringComparison.Ordinal)));
            }

            return profiles;
        }

        return profiles
            .Select(p => seenNames[p.Name] > 1 ? p with { Name = $"{p.Name} ({p.Directory})" } : p)
            .ToList();
    }

    public static async Task<IReadOnlyList<HistoryEntry>> GetHistoryAsync(string userDataDir, HistoryRequest request, CancellationToken cancellationToken = default)
    {
        if (!DateOnly.TryParseExact(request.Date, "yyyy-MM-dd", CultureInfo.InvariantCulture, DateTimeStyles.None, out var date))
        {
            throw new EdgeHistoryException($"Invalid date '{request.Date}'; expected yyyy-MM-dd.");
        }

        var profiles = ListProfiles(userDataDir);
        var profile = profiles.FirstOrDefault(p => string.Equals(p.Name, request.Profile, StringComparison.Ordinal) || string.Equals(p.Directory, request.Profile, StringComparison.Ordinal));
        if (profile is null)
        {
            throw new EdgeHistoryException($"Unknown profile '{request.Profile}'.");
        }

        var (startUtc, endUtc) = LocalTimeRange.ToUtcRange(date, request.StartTime, request.EndTime);
        var startChrome = DateTimeConverters.DateTimeOffsetToChromeMicroseconds(startUtc);
        var endChrome = DateTimeConverters.DateTimeOffsetToChromeMicroseconds(endUtc);

        var baseDir = Path.GetFullPath(userDataDir);
        var historyPath = Path.GetFullPath(Path.Combine(baseDir, profile.Directory, "History"));
        if (!historyPath.StartsWith(baseDir, StringComparison.Ordinal))
        {
            throw new EdgeHistoryException("Resolved history path escapes the Edge User Data directory.");
        }

        using var snapshot = HistorySnapshot.Create(historyPath);

        return await Task.Run(() =>
        {
            cancellationToken.ThrowIfCancellationRequested();

            using var conn = new SQLiteConnection($"Data Source={snapshot.HistoryPath};Version=3;Read Only=True;");
            conn.Open();

            using var busy = conn.CreateCommand();
            busy.CommandText = "PRAGMA busy_timeout = 5000";
            busy.ExecuteNonQuery();

            using var command = conn.CreateCommand();
            command.CommandText = @"
                SELECT v.id, v.visit_time, v.transition,
                       u.id, u.url, u.title, u.visit_count, u.typed_count
                FROM visits v
                JOIN urls u ON v.url = u.id
                WHERE v.visit_time >= @start AND v.visit_time < @end
                ORDER BY v.visit_time ASC";

            command.Parameters.AddWithValue("@start", startChrome);
            command.Parameters.AddWithValue("@end", endChrome);

            var entries = new List<HistoryEntry>();
            using var reader = command.ExecuteReader();
            while (reader.Read())
            {
                cancellationToken.ThrowIfCancellationRequested();

                var chromeMicroseconds = reader.GetInt64(1);
                var visitTime = DateTimeConverters.TryChromeMicrosecondsToLocalDateTimeOffset(chromeMicroseconds);
                if (visitTime is null)
                {
                    continue; // skip entries with corrupt timestamps
                }

                entries.Add(new HistoryEntry(
                    VisitTime: visitTime.Value.ToString("O", CultureInfo.InvariantCulture),
                    Url: reader.IsDBNull(4) ? string.Empty : reader.GetString(4),
                    Title: reader.IsDBNull(5) ? string.Empty : reader.GetString(5),
                    VisitCount: reader.IsDBNull(6) ? 0 : reader.GetInt32(6),
                    TypedCount: reader.IsDBNull(7) ? 0 : reader.GetInt32(7),
                    Transition: DecodeTransition(reader.IsDBNull(2) ? 0 : reader.GetInt32(2)),
                    UrlId: reader.IsDBNull(3) ? 0 : reader.GetInt64(3),
                    VisitId: reader.IsDBNull(0) ? 0 : reader.GetInt64(0)));
            }

            return (IReadOnlyList<HistoryEntry>)entries;
        }, cancellationToken);
    }

    private const long MaxLocalStateSize = 10 * 1024 * 1024; // 10 MB

    private static Dictionary<string, string?> LoadInfoCache(string localStatePath)
    {
        if (!File.Exists(localStatePath))
        {
            return new Dictionary<string, string?>();
        }

        try
        {
            var fileInfo = new FileInfo(localStatePath);
            if (fileInfo.Length > MaxLocalStateSize)
            {
                return new Dictionary<string, string?>();
            }

            using var stream = File.OpenRead(localStatePath);
            using var doc = JsonDocument.Parse(stream);
            if (!doc.RootElement.TryGetProperty("profile", out var profileElement) ||
                !profileElement.TryGetProperty("info_cache", out var infoCacheElement) ||
                infoCacheElement.ValueKind != JsonValueKind.Object)
            {
                return new Dictionary<string, string?>();
            }

            var result = new Dictionary<string, string?>(StringComparer.Ordinal);
            foreach (var property in infoCacheElement.EnumerateObject())
            {
                string? name = null;
                if (property.Value.ValueKind == JsonValueKind.Object &&
                    property.Value.TryGetProperty("name", out var nameElement) &&
                    nameElement.ValueKind == JsonValueKind.String)
                {
                    name = nameElement.GetString();
                }

                result[property.Name] = name;
            }

            return result;
        }
        catch (JsonException)
        {
            return new Dictionary<string, string?>();
        }
        catch (IOException)
        {
            return new Dictionary<string, string?>();
        }
    }

    private static bool IsSafeProfileDirectory(string directory)
    {
        if (string.IsNullOrWhiteSpace(directory) || directory is "." or "..")
        {
            return false;
        }

        if (directory.Contains('/') || directory.Contains('\\'))
        {
            return false;
        }

        return !Path.IsPathRooted(directory);
    }

    private static string DecodeTransition(int transition)
    {
        var core = transition & 0xFF;
        return TransitionMap.TryGetValue(core, out var decoded) ? decoded : "unknown";
    }
}

public static class LocalTimeRange
{
    public static (DateTimeOffset StartUtc, DateTimeOffset EndUtc) ToUtcRange(DateOnly day, string? startTime, string? endTime)
    {
        var start = startTime is null
            ? day.ToDateTime(TimeOnly.MinValue)
            : day.ToDateTime(ParseTime(startTime));

        var end = endTime is null
            ? day.AddDays(1).ToDateTime(TimeOnly.MinValue)
            : day.ToDateTime(ParseTime(endTime));

        if (start >= end)
        {
            throw new EdgeHistoryException("Invalid time range: --start-time must be earlier than --end-time.");
        }

        var localZone = TimeZoneInfo.Local;
        var startUtc = new DateTimeOffset(TimeZoneInfo.ConvertTimeToUtc(DateTime.SpecifyKind(start, DateTimeKind.Unspecified), localZone), TimeSpan.Zero);
        var endUtc = new DateTimeOffset(TimeZoneInfo.ConvertTimeToUtc(DateTime.SpecifyKind(end, DateTimeKind.Unspecified), localZone), TimeSpan.Zero);
        return (startUtc, endUtc);
    }

    private static TimeOnly ParseTime(string value)
    {
        if (TimeOnly.TryParseExact(value, ["HH:mm", "HH:mm:ss"], CultureInfo.InvariantCulture, DateTimeStyles.None, out var parsed))
        {
            return parsed;
        }

        throw new EdgeHistoryException($"Invalid time '{value}'; expected HH:mm or HH:mm:ss.");
    }
}

public static class DateTimeConverters
{
    private const long ChromeEpochOffsetMicroseconds = 11_644_473_600L * 1_000_000L;
    private static readonly DateTimeOffset ChromeEpoch = new(1601, 1, 1, 0, 0, 0, TimeSpan.Zero);

    // Valid Chrome timestamp range: 1601-01-01 to 9999-12-31
    private const long MinChromeMicroseconds = 0;
    private const long MaxChromeMicroseconds = 265_046_774_399_999_999L; // approx year 9999

    public static DateTimeOffset ChromeMicrosecondsToLocalDateTimeOffset(long chromeMicroseconds)
    {
        if (chromeMicroseconds < MinChromeMicroseconds || chromeMicroseconds > MaxChromeMicroseconds)
        {
            throw new ArgumentOutOfRangeException(nameof(chromeMicroseconds), "Chrome timestamp is out of valid range.");
        }

        var utc = ChromeEpoch.AddTicks(chromeMicroseconds * 10);
        return TimeZoneInfo.ConvertTime(utc, TimeZoneInfo.Local);
    }

    public static DateTimeOffset? TryChromeMicrosecondsToLocalDateTimeOffset(long chromeMicroseconds)
    {
        if (chromeMicroseconds < MinChromeMicroseconds || chromeMicroseconds > MaxChromeMicroseconds)
        {
            return null;
        }

        try
        {
            var utc = ChromeEpoch.AddTicks(chromeMicroseconds * 10);
            return TimeZoneInfo.ConvertTime(utc, TimeZoneInfo.Local);
        }
        catch (ArgumentOutOfRangeException)
        {
            return null;
        }
    }

    public static long DateTimeOffsetToChromeMicroseconds(DateTimeOffset value)
    {
        var utc = value.ToUniversalTime();
        var ticks = utc.Ticks - ChromeEpoch.Ticks;
        return ticks / 10;
    }
}

internal sealed class HistorySnapshot : IDisposable
{
    private readonly string _tempDirectory;

    private HistorySnapshot(string tempDirectory, string historyPath)
    {
        _tempDirectory = tempDirectory;
        HistoryPath = historyPath;
    }

    public string HistoryPath { get; }

    public static HistorySnapshot Create(string sourceHistoryPath)
    {
        var tempDirectory = Directory.CreateTempSubdirectory("edge_history_cli_").FullName;
        var copiedHistoryPath = Path.Combine(tempDirectory, "History");

        try
        {
            File.Copy(sourceHistoryPath, copiedHistoryPath, overwrite: true);
            foreach (var suffix in new[] { "-journal", "-wal", "-shm" })
            {
                var sourceSidecar = sourceHistoryPath + suffix;
                if (File.Exists(sourceSidecar))
                {
                    File.Copy(sourceSidecar, copiedHistoryPath + suffix, overwrite: true);
                }
            }
        }
        catch (IOException ex)
        {
            throw new EdgeHistoryException($"Unable to copy Edge history database: {ex.Message}");
        }

        return new HistorySnapshot(tempDirectory, copiedHistoryPath);
    }

    public void Dispose()
    {
        try
        {
            Directory.Delete(_tempDirectory, recursive: true);
        }
        catch (IOException ex)
        {
            Console.Error.WriteLine($"Warning: Failed to clean up temp directory '{_tempDirectory}': {ex.Message}");
        }
        catch (UnauthorizedAccessException ex)
        {
            Console.Error.WriteLine($"Warning: Failed to clean up temp directory '{_tempDirectory}': {ex.Message}");
        }
    }
}
