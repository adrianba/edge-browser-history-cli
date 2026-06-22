using System.Data.SQLite;
namespace EdgeBrowserHistoryCli.Tests;

public class CliAndHistoryTests
{
    [Fact]
    public void ParseProfilesCommand_Works()
    {
        var parsed = CliArguments.Parse(new[] { "--profiles", "--user-data-dir", "/tmp/edge" });

        Assert.True(parsed.ListProfiles);
        Assert.Null(parsed.HistoryRequest);
        Assert.Equal("/tmp/edge", parsed.UserDataDir);
    }

    [Fact]
    public void ParseHistoryWithoutProfile_Throws()
    {
        var ex = Assert.Throws<EdgeHistoryException>(() => CliArguments.Parse(new[] { "--history", "--date", "2026-01-01" }));
        Assert.Contains("--profile", ex.Message);
    }

    [Fact]
    public void LocalTimeRange_InvalidRange_Throws()
    {
        var ex = Assert.Throws<EdgeHistoryException>(() => LocalTimeRange.ToUtcRange(new DateOnly(2026, 1, 1), "10:00", "09:59"));
        Assert.Contains("Invalid time range", ex.Message);
    }

    [Fact]
    public async Task HistoryQuery_FiltersByProfileDateAndTimeRange()
    {
        var root = Path.Combine(Path.GetTempPath(), $"edge-history-cli-tests-{Guid.NewGuid():N}");
        Directory.CreateDirectory(root);

        try
        {
            var defaultDir = Path.Combine(root, "Default");
            Directory.CreateDirectory(defaultDir);
            File.WriteAllText(Path.Combine(root, "Local State"), """
                {
                  "profile": {
                    "info_cache": {
                      "Default": { "name": "Personal" }
                    }
                  }
                }
                """);

            var historyPath = Path.Combine(defaultDir, "History");
            CreateHistoryDb(historyPath, new DateOnly(2026, 1, 1));

            var profiles = EdgeHistoryService.ListProfiles(root);
            Assert.Single(profiles);
            Assert.Equal("Personal", profiles[0].Name);

            var entries = await EdgeHistoryService.GetHistoryAsync(root, new HistoryRequest("Personal", "2026-01-01", "09:00", "10:00"));
            var entry = Assert.Single(entries);
            Assert.Equal("https://example.com/one", entry.Url);
        }
        finally
        {
            if (Directory.Exists(root))
            {
                Directory.Delete(root, recursive: true);
            }
        }
    }

    private static void CreateHistoryDb(string path, DateOnly day)
    {
        SQLiteConnection.CreateFile(path);
        using var conn = new SQLiteConnection($"Data Source={path};Version=3;");
        conn.Open();

        using (var cmd = conn.CreateCommand())
        {
            cmd.CommandText = """
                CREATE TABLE urls (
                    id INTEGER PRIMARY KEY,
                    url LONGVARCHAR,
                    title LONGVARCHAR,
                    visit_count INTEGER,
                    typed_count INTEGER
                );
                CREATE TABLE visits (
                    id INTEGER PRIMARY KEY,
                    url INTEGER,
                    visit_time INTEGER,
                    transition INTEGER
                );
                """;
            cmd.ExecuteNonQuery();
        }

        InsertVisit(conn, 1, "https://example.com/one", "One", LocalToChrome(day, new TimeOnly(9, 30, 0)), 1);
        InsertVisit(conn, 2, "https://example.com/two", "Two", LocalToChrome(day, new TimeOnly(11, 0, 0)), 8);
    }

    private static void InsertVisit(SQLiteConnection conn, int id, string url, string title, long visitTime, int transition)
    {
        using var cmd = conn.CreateCommand();
        cmd.CommandText = """
            INSERT INTO urls(id, url, title, visit_count, typed_count) VALUES(@id, @url, @title, 1, 0);
            INSERT INTO visits(id, url, visit_time, transition) VALUES(@id, @id, @visitTime, @transition);
            """;
        cmd.Parameters.AddWithValue("@id", id);
        cmd.Parameters.AddWithValue("@url", url);
        cmd.Parameters.AddWithValue("@title", title);
        cmd.Parameters.AddWithValue("@visitTime", visitTime);
        cmd.Parameters.AddWithValue("@transition", transition);
        cmd.ExecuteNonQuery();
    }

    private static long LocalToChrome(DateOnly day, TimeOnly time)
    {
        var local = day.ToDateTime(time, DateTimeKind.Unspecified);
        var utc = TimeZoneInfo.ConvertTimeToUtc(local, TimeZoneInfo.Local);
        return DateTimeConverters.DateTimeOffsetToChromeMicroseconds(new DateTimeOffset(utc, TimeSpan.Zero));
    }
}
