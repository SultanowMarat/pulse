using System;
using System.ComponentModel;
using System.Diagnostics;
using System.IO;
using System.Net.Http;
using System.Security.Cryptography;
using System.Text.Json;

namespace Pulse.Desktop;

internal static class AutoUpdater
{
    private static readonly TimeSpan ManifestTimeout = TimeSpan.FromSeconds(3);
    private static readonly TimeSpan DownloadTimeout = TimeSpan.FromMinutes(2);

    public static bool TryApplyUpdateOnStartup()
    {
        if (!IsAutoUpdateEnabled())
        {
            return false;
        }

        if (!Uri.TryCreate(WrapperSettings.UpdateManifestUrl, UriKind.Absolute, out var manifestUri))
        {
            return false;
        }

        if (!TryParseVersion(WrapperSettings.AppVersion, out var currentVersion))
        {
            return false;
        }

        try
        {
            var manifest = FetchManifest(manifestUri);
            if (manifest is null || !TryParseVersion(manifest.Version, out var latestVersion))
            {
                return false;
            }

            if (latestVersion <= currentVersion)
            {
                return false;
            }

            if (!TryBuildAbsoluteUri(manifest.MsiUrl, manifestUri, out var msiUri))
            {
                return false;
            }

            var updateDirectory = BuildUpdateDirectory();
            Directory.CreateDirectory(updateDirectory);

            var msiPath = Path.Combine(updateDirectory, $"PulseDesktop-{latestVersion}.msi");
            if (!DownloadMsi(msiUri, msiPath))
            {
                return false;
            }

            if (!ValidateSha256IfProvided(msiPath, manifest.Sha256))
            {
                return false;
            }

            CleanupOldPackages(updateDirectory, msiPath);
            return LaunchInstaller(msiPath);
        }
        catch
        {
            return false;
        }
    }

    private static bool IsAutoUpdateEnabled()
    {
        return bool.TryParse(WrapperSettings.AutoUpdateEnabled, out var enabled) && enabled;
    }

    private static UpdateManifest? FetchManifest(Uri manifestUri)
    {
        using var client = CreateHttpClient(ManifestTimeout);
        using var response = client.GetAsync(manifestUri).GetAwaiter().GetResult();
        if (!response.IsSuccessStatusCode)
        {
            return null;
        }

        var body = response.Content.ReadAsStringAsync().GetAwaiter().GetResult();
        using var document = JsonDocument.Parse(body);
        var root = document.RootElement;
        if (root.ValueKind != JsonValueKind.Object)
        {
            return null;
        }

        if (root.TryGetProperty("windows", out var windowsNode) && windowsNode.ValueKind == JsonValueKind.Object)
        {
            root = windowsNode;
        }

        var version = GetFirstString(root, "version", "appVersion", "latestVersion");
        var msiUrl = GetFirstString(root, "msi_url", "msiUrl", "download_url", "downloadUrl", "url");
        var sha256 = GetFirstString(root, "sha256", "sha_256");

        if (string.IsNullOrWhiteSpace(version) || string.IsNullOrWhiteSpace(msiUrl))
        {
            return null;
        }

        return new UpdateManifest(version.Trim(), msiUrl.Trim(), sha256?.Trim());
    }

    private static string? GetFirstString(JsonElement root, params string[] names)
    {
        foreach (var name in names)
        {
            if (!root.TryGetProperty(name, out var prop))
            {
                continue;
            }

            if (prop.ValueKind == JsonValueKind.String)
            {
                var value = prop.GetString();
                if (!string.IsNullOrWhiteSpace(value))
                {
                    return value;
                }
            }
        }

        return null;
    }

    private static bool TryBuildAbsoluteUri(string rawUrl, Uri manifestUri, out Uri uri)
    {
        if (Uri.TryCreate(rawUrl, UriKind.Absolute, out uri!))
        {
            return uri.Scheme is "http" or "https";
        }

        if (Uri.TryCreate(manifestUri, rawUrl, out uri!))
        {
            return uri.Scheme is "http" or "https";
        }

        uri = null!;
        return false;
    }

    private static bool DownloadMsi(Uri msiUri, string destinationPath)
    {
        using var client = CreateHttpClient(DownloadTimeout);
        using var response = client.GetAsync(msiUri, HttpCompletionOption.ResponseHeadersRead).GetAwaiter().GetResult();
        if (!response.IsSuccessStatusCode)
        {
            return false;
        }

        var tempPath = destinationPath + ".download";
        if (File.Exists(tempPath))
        {
            File.Delete(tempPath);
        }

        using (var source = response.Content.ReadAsStream())
        using (var destination = new FileStream(tempPath, FileMode.Create, FileAccess.Write, FileShare.None))
        {
            source.CopyTo(destination);
        }

        if (File.Exists(destinationPath))
        {
            File.Delete(destinationPath);
        }

        File.Move(tempPath, destinationPath);
        return true;
    }

    private static bool ValidateSha256IfProvided(string filePath, string? expectedSha256)
    {
        if (string.IsNullOrWhiteSpace(expectedSha256))
        {
            return true;
        }

        var normalizedExpected = expectedSha256.Replace("-", string.Empty).Trim().ToUpperInvariant();
        using var sha = SHA256.Create();
        using var stream = File.OpenRead(filePath);
        var hash = sha.ComputeHash(stream);
        var actual = Convert.ToHexString(hash).ToUpperInvariant();
        return string.Equals(actual, normalizedExpected, StringComparison.Ordinal);
    }

    private static string BuildUpdateDirectory()
    {
        static string Safe(string value)
        {
            var safe = value.Trim();
            foreach (var ch in Path.GetInvalidFileNameChars())
            {
                safe = safe.Replace(ch, '_');
            }

            return string.IsNullOrWhiteSpace(safe) ? "PulseDesktop" : safe;
        }

        return Path.Combine(
            Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),
            Safe(WrapperSettings.CompanyName),
            Safe(WrapperSettings.AppName),
            "Updates");
    }

    private static void CleanupOldPackages(string directory, string keepPath)
    {
        try
        {
            var keepFile = Path.GetFullPath(keepPath);
            foreach (var file in Directory.GetFiles(directory, "*.msi"))
            {
                if (!string.Equals(Path.GetFullPath(file), keepFile, StringComparison.OrdinalIgnoreCase))
                {
                    File.Delete(file);
                }
            }
        }
        catch
        {
            // Best-effort cleanup only.
        }
    }

    private static bool LaunchInstaller(string msiPath)
    {
        var args = $"/i \"{msiPath}\" /passive /norestart";
        var startInfo = new ProcessStartInfo("msiexec.exe", args)
        {
            UseShellExecute = true,
            Verb = "runas",
            WindowStyle = ProcessWindowStyle.Hidden,
        };

        try
        {
            var process = Process.Start(startInfo);
            return process is not null;
        }
        catch (Win32Exception ex) when (ex.NativeErrorCode == 1223)
        {
            // UAC was cancelled.
            return false;
        }
        catch
        {
            return false;
        }
    }

    private static HttpClient CreateHttpClient(TimeSpan timeout)
    {
        var client = new HttpClient
        {
            Timeout = timeout,
        };
        client.DefaultRequestHeaders.UserAgent.ParseAdd($"PulseDesktop/{WrapperSettings.AppVersion}");
        return client;
    }

    private static bool TryParseVersion(string raw, out Version version)
    {
        var value = (raw ?? string.Empty).Trim();
        if (Version.TryParse(value, out version!))
        {
            return true;
        }

        var clean = value.Split('-', '+', StringSplitOptions.RemoveEmptyEntries)[0];
        return Version.TryParse(clean, out version!);
    }

    private sealed record UpdateManifest(string Version, string MsiUrl, string? Sha256);
}
