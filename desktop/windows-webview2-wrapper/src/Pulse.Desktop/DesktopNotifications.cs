using System;
using System.Collections.Generic;
using System.Linq;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Input;
using System.Windows.Media;
using System.Windows.Media.Imaging;
using WpfBrushes = System.Windows.Media.Brushes;
using WpfColor = System.Windows.Media.Color;

namespace Pulse.Desktop;

internal sealed class DesktopNotificationManager : IDisposable
{
    private const int MaxVisibleNotifications = 4;
    private static readonly TimeSpan DuplicateWindow = TimeSpan.FromSeconds(2);
    private readonly Queue<DesktopNotificationPayload> _pending = new();
    private readonly List<DesktopToastWindow> _visible = new();
    private readonly Dictionary<string, DateTime> _recentKeys = new(StringComparer.Ordinal);
    private readonly Action<DesktopNotificationPayload> _onClick;
    private bool _isDisposed;

    public DesktopNotificationManager(Action<DesktopNotificationPayload> onClick)
    {
        _onClick = onClick;
    }

    public void Enqueue(DesktopNotificationPayload payload)
    {
        if (_isDisposed)
        {
            return;
        }

        if (IsDuplicate(payload))
        {
            return;
        }

        _pending.Enqueue(payload);
        TryShowQueued();
    }

    public void Dispose()
    {
        if (_isDisposed)
        {
            return;
        }

        _isDisposed = true;

        foreach (var toast in _visible.ToList())
        {
            toast.CloseSilently();
        }

        _visible.Clear();
        _pending.Clear();
        _recentKeys.Clear();
    }

    public void DismissByChatId(string? chatId)
    {
        if (_isDisposed || string.IsNullOrWhiteSpace(chatId))
        {
            return;
        }

        var normalized = chatId.Trim();
        if (_pending.Count > 0)
        {
            var filtered = _pending
                .Where(p => !string.Equals((p.ChatId ?? string.Empty).Trim(), normalized, StringComparison.Ordinal))
                .ToArray();
            _pending.Clear();
            foreach (var payload in filtered)
            {
                _pending.Enqueue(payload);
            }
        }

        foreach (var toast in _visible
                     .Where(v => string.Equals((v.ChatId ?? string.Empty).Trim(), normalized, StringComparison.Ordinal))
                     .ToList())
        {
            toast.CloseSilently();
        }
    }

    public void DismissAll()
    {
        if (_isDisposed)
        {
            return;
        }

        _pending.Clear();
        foreach (var toast in _visible.ToList())
        {
            toast.CloseSilently();
        }
    }

    private void TryShowQueued()
    {
        while (_visible.Count < MaxVisibleNotifications && _pending.Count > 0)
        {
            var payload = _pending.Dequeue();
            var toast = new DesktopToastWindow(payload, HandleNotificationClick, HandleToastClosed);
            _visible.Add(toast);
            RepositionToasts();
            toast.Show();
        }
    }

    private void HandleNotificationClick(DesktopNotificationPayload payload)
    {
        _onClick(payload);
    }

    private void HandleToastClosed(DesktopToastWindow toast)
    {
        _visible.Remove(toast);
        RepositionToasts();
        TryShowQueued();
    }

    private void RepositionToasts()
    {
        for (var i = 0; i < _visible.Count; i++)
        {
            var orderFromBottom = _visible.Count - 1 - i;
            _visible[i].Reposition(orderFromBottom);
        }
    }

    private bool IsDuplicate(DesktopNotificationPayload payload)
    {
        var key = GetDedupKey(payload);
        var now = DateTime.UtcNow;

        var expired = _recentKeys
            .Where(x => now - x.Value > DuplicateWindow)
            .Select(x => x.Key)
            .ToList();
        foreach (var oldKey in expired)
        {
            _recentKeys.Remove(oldKey);
        }

        if (_recentKeys.TryGetValue(key, out var seenAt) && now - seenAt <= DuplicateWindow)
        {
            return true;
        }

        if (_pending.Any(p => string.Equals(GetDedupKey(p), key, StringComparison.Ordinal)))
        {
            return true;
        }

        if (_visible.Any(v => string.Equals(v.DedupKey, key, StringComparison.Ordinal)))
        {
            return true;
        }

        _recentKeys[key] = now;
        return false;
    }

    internal static string GetDedupKey(DesktopNotificationPayload payload)
    {
        static string N(string? v) => (v ?? string.Empty).Trim();
        return $"{N(payload.ChatId)}|{N(payload.Sender)}|{N(payload.Body)}";
    }
}

internal sealed class DesktopToastWindow : Window
{
    private readonly DesktopNotificationPayload _payload;
    private readonly Action<DesktopNotificationPayload> _onClick;
    private readonly Action<DesktopToastWindow> _onClosed;
    public string DedupKey { get; }
    public string? ChatId => _payload.ChatId;

    public DesktopToastWindow(
        DesktopNotificationPayload payload,
        Action<DesktopNotificationPayload> onClick,
        Action<DesktopToastWindow> onClosed)
    {
        _payload = payload;
        _onClick = onClick;
        _onClosed = onClosed;
        DedupKey = DesktopNotificationManager.GetDedupKey(payload);

        Width = 420;
        Height = 112;
        ShowActivated = false;
        ShowInTaskbar = false;
        ResizeMode = ResizeMode.NoResize;
        WindowStyle = WindowStyle.None;
        AllowsTransparency = true;
        Background = WpfBrushes.Transparent;
        Topmost = true;

        Content = BuildContent(payload);
        Closed += OnClosed;
    }

    public void Reposition(int orderFromBottom)
    {
        const int rightMargin = 14;
        const int bottomMargin = 14;
        const int spacing = 10;

        var workArea = SystemParameters.WorkArea;
        Left = workArea.Right - Width - rightMargin;
        Top = workArea.Bottom - Height - bottomMargin - ((Height + spacing) * orderFromBottom);
    }

    public void CloseSilently()
    {
        Close();
    }

    private UIElement BuildContent(DesktopNotificationPayload payload)
    {
        var border = new Border
        {
            Background = new SolidColorBrush(WpfColor.FromRgb(32, 35, 42)),
            BorderBrush = new SolidColorBrush(WpfColor.FromRgb(56, 61, 73)),
            BorderThickness = new Thickness(1),
            CornerRadius = new CornerRadius(12),
            Padding = new Thickness(12),
            SnapsToDevicePixels = true,
        };

        var layout = new Grid();
        layout.ColumnDefinitions.Add(new ColumnDefinition { Width = GridLength.Auto });
        layout.ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(1, GridUnitType.Star) });
        layout.ColumnDefinitions.Add(new ColumnDefinition { Width = GridLength.Auto });
        layout.RowDefinitions.Add(new RowDefinition { Height = GridLength.Auto });
        layout.RowDefinitions.Add(new RowDefinition { Height = GridLength.Auto });
        layout.RowDefinitions.Add(new RowDefinition { Height = GridLength.Auto });

        var avatar = BuildAvatarElement(payload);
        Grid.SetColumn(avatar, 0);
        Grid.SetRow(avatar, 0);
        Grid.SetRowSpan(avatar, 3);
        layout.Children.Add(avatar);

        var title = new TextBlock
        {
            Text = payload.Title,
            Foreground = new SolidColorBrush(WpfColor.FromRgb(244, 246, 250)),
            FontSize = 14,
            FontWeight = FontWeights.SemiBold,
            TextTrimming = TextTrimming.CharacterEllipsis,
            Margin = new Thickness(12, 0, 10, 0),
            VerticalAlignment = VerticalAlignment.Center,
        };
        Grid.SetColumn(title, 1);
        Grid.SetRow(title, 0);
        layout.Children.Add(title);

        var closeButton = new System.Windows.Controls.Button
        {
            Content = "x",
            Width = 22,
            Height = 22,
            FontSize = 14,
            FontWeight = FontWeights.Bold,
            Cursor = System.Windows.Input.Cursors.Hand,
            Margin = new Thickness(4, 0, 0, 0),
            HorizontalAlignment = System.Windows.HorizontalAlignment.Right,
            VerticalAlignment = VerticalAlignment.Top,
            Foreground = new SolidColorBrush(WpfColor.FromRgb(189, 196, 206)),
            Background = WpfBrushes.Transparent,
            BorderBrush = WpfBrushes.Transparent,
            Padding = new Thickness(0),
        };
        closeButton.PreviewMouseLeftButtonUp += (_, e) =>
        {
            e.Handled = true;
            Close();
        };
        Grid.SetColumn(closeButton, 2);
        Grid.SetRow(closeButton, 0);
        layout.Children.Add(closeButton);

        if (!string.IsNullOrWhiteSpace(payload.Sender))
        {
            var sender = new TextBlock
            {
                Text = payload.Sender,
                Foreground = new SolidColorBrush(WpfColor.FromRgb(109, 180, 255)),
                FontSize = 13,
                FontWeight = FontWeights.Medium,
                TextTrimming = TextTrimming.CharacterEllipsis,
                Margin = new Thickness(12, 3, 6, 0),
            };
            Grid.SetColumn(sender, 1);
            Grid.SetRow(sender, 1);
            Grid.SetColumnSpan(sender, 2);
            layout.Children.Add(sender);
        }

        var body = new TextBlock
        {
            Text = payload.Body,
            Foreground = new SolidColorBrush(WpfColor.FromRgb(204, 211, 220)),
            FontSize = 13,
            Margin = new Thickness(12, 3, 6, 0),
            TextWrapping = TextWrapping.Wrap,
            TextTrimming = TextTrimming.CharacterEllipsis,
            MaxHeight = 40,
        };
        Grid.SetColumn(body, 1);
        Grid.SetRow(body, 2);
        Grid.SetColumnSpan(body, 2);
        layout.Children.Add(body);

        border.Child = layout;
        border.MouseLeftButtonUp += OnToastMouseLeftButtonUp;
        return border;
    }

    private static FrameworkElement BuildAvatarElement(DesktopNotificationPayload payload)
    {
        var avatarSize = 48.0;
        var initials = GetInitials(payload.Sender, payload.Title);

        var host = new Grid
        {
            Width = avatarSize,
            Height = avatarSize,
        };

        var fallback = new Border
        {
            Width = avatarSize,
            Height = avatarSize,
            CornerRadius = new CornerRadius(avatarSize / 2),
            Background = new SolidColorBrush(GetAvatarColor(payload.Sender, payload.Title)),
            Child = new TextBlock
            {
                Text = initials,
                Foreground = WpfBrushes.White,
                FontSize = 18,
                FontWeight = FontWeights.SemiBold,
                HorizontalAlignment = System.Windows.HorizontalAlignment.Center,
                VerticalAlignment = VerticalAlignment.Center,
            },
        };
        host.Children.Add(fallback);

        var avatarUri = TryCreateAvatarUri(payload.AvatarUrl);
        if (avatarUri is null)
        {
            return host;
        }

        try
        {
            var bitmap = new BitmapImage();
            bitmap.BeginInit();
            bitmap.UriSource = avatarUri;
            bitmap.CacheOption = BitmapCacheOption.OnLoad;
            bitmap.EndInit();

            var image = new System.Windows.Controls.Image
            {
                Width = avatarSize,
                Height = avatarSize,
                Source = bitmap,
                Stretch = Stretch.UniformToFill,
                Clip = new EllipseGeometry(new System.Windows.Point(avatarSize / 2, avatarSize / 2), avatarSize / 2, avatarSize / 2),
            };
            host.Children.Add(image);
        }
        catch
        {
            // Keep initials fallback.
        }

        return host;
    }

    private static Uri? TryCreateAvatarUri(string? rawUrl)
    {
        if (string.IsNullOrWhiteSpace(rawUrl))
        {
            return null;
        }

        var trimmed = rawUrl.Trim();
        if (!Uri.TryCreate(trimmed, UriKind.Absolute, out var uri))
        {
            return null;
        }

        return uri.Scheme is "http" or "https" ? uri : null;
    }

    private static string GetInitials(string? sender, string title)
    {
        var source = string.IsNullOrWhiteSpace(sender) ? title : sender.Trim();
        if (string.IsNullOrWhiteSpace(source))
        {
            return "P";
        }

        var parts = source.Split(' ', StringSplitOptions.RemoveEmptyEntries);
        if (parts.Length >= 2)
        {
            return $"{char.ToUpperInvariant(parts[0][0])}{char.ToUpperInvariant(parts[1][0])}";
        }

        return source.Length >= 2
            ? source[..2].ToUpperInvariant()
            : source.ToUpperInvariant();
    }

    private static WpfColor GetAvatarColor(string? sender, string title)
    {
        var source = string.IsNullOrWhiteSpace(sender) ? title : sender;
        unchecked
        {
            var hash = 17;
            foreach (var c in source ?? string.Empty)
            {
                hash = hash * 31 + c;
            }

            byte r = (byte)(90 + (Math.Abs(hash) % 90));
            byte g = (byte)(110 + (Math.Abs(hash / 13) % 90));
            byte b = (byte)(130 + (Math.Abs(hash / 29) % 90));
            return WpfColor.FromRgb(r, g, b);
        }
    }

    private void OnToastMouseLeftButtonUp(object sender, MouseButtonEventArgs e)
    {
        _onClick(_payload);
        Close();
    }

    private void OnClosed(object? sender, EventArgs e)
    {
        _onClosed(this);
    }
}

internal sealed record DesktopNotificationPayload(
    string Title,
    string Body,
    string? ChatId,
    string? Sender,
    string? AvatarUrl);
