using System;
using System.IO;
using System.Text.Json;
using System.Threading.Tasks;
using System.Windows;
using System.Windows.Input;
using System.Windows.Interop;
using Microsoft.Web.WebView2.Core;
using Drawing = System.Drawing;
using Forms = System.Windows.Forms;

namespace Pulse.Desktop;

public partial class MainWindow : Window
{
    private readonly DesktopNotificationManager _notificationManager;
    private readonly Forms.NotifyIcon _trayIcon;

    private bool _isFullScreen;
    private bool _webBridgeInitialized;
    private bool _exitRequested;
    private bool _restoringWebView;
    private bool _isHidingToTray;
    private bool _isParkedInTray;
    private Rect _restoreBoundsBeforeTray;
    private WindowState _windowStateBeforeTray = WindowState.Maximized;

    private WindowStyle _savedWindowStyle;
    private ResizeMode _savedResizeMode;
    private WindowState _savedWindowState;
    private Rect _savedRestoreBounds;
    private Uri? _lastAttemptedUri;

    private static readonly JsonSerializerOptions BridgeJsonOptions = new()
    {
        PropertyNameCaseInsensitive = true,
    };

    private const string DesktopBridgeScript = """
(() => {
  if (window.__pulseDesktopBridgeInstalled) {
    return;
  }
  window.__pulseDesktopBridgeInstalled = true;

  let lastIncomingChatId = null;

  const trackIncomingMessage = (rawData) => {
    try {
      const parsed = typeof rawData === 'string' ? JSON.parse(rawData) : rawData;
      if (parsed && parsed.type === 'new_message' && parsed.payload && parsed.payload.chat_id) {
        lastIncomingChatId = String(parsed.payload.chat_id);
      }
    } catch {
      // Ignore malformed messages.
    }
  };

  const wsProto = WebSocket.prototype;
  const originalAddEventListener = wsProto.addEventListener;
  wsProto.addEventListener = function(type, listener, options) {
    if (type === 'message' && typeof listener === 'function') {
      const wrapped = function(event) {
        trackIncomingMessage(event.data);
        return listener.call(this, event);
      };
      return originalAddEventListener.call(this, type, wrapped, options);
    }
    return originalAddEventListener.call(this, type, listener, options);
  };

  const onMessageDescriptor = Object.getOwnPropertyDescriptor(wsProto, 'onmessage');
  if (onMessageDescriptor && onMessageDescriptor.set && onMessageDescriptor.get) {
    Object.defineProperty(wsProto, 'onmessage', {
      configurable: true,
      enumerable: onMessageDescriptor.enumerable,
      get: function() {
        return onMessageDescriptor.get.call(this);
      },
      set: function(handler) {
        if (typeof handler !== 'function') {
          onMessageDescriptor.set.call(this, handler);
          return;
        }
        const wrapped = function(event) {
          trackIncomingMessage(event.data);
          return handler.call(this, event);
        };
        onMessageDescriptor.set.call(this, wrapped);
      },
    });
  }

  const sendToHost = (payload) => {
    try {
      if (window.chrome && window.chrome.webview) {
        window.chrome.webview.postMessage(payload);
      }
    } catch {
      // Ignore bridge failures.
    }
  };

  window.electronAPI = {
    showNotification: (opts) => {
      const safe = opts && typeof opts === 'object' ? opts : {};
      const title = typeof safe.title === 'string' && safe.title.trim() ? safe.title : 'Pulse';
      const body = typeof safe.body === 'string' ? safe.body : '';
      const chatId = safe.chatId ? String(safe.chatId) : lastIncomingChatId;
      const sender = typeof safe.sender === 'string' ? safe.sender : '';
      const avatarUrl = typeof safe.avatarUrl === 'string' ? safe.avatarUrl : '';
      sendToHost({ type: 'show-notification', title, body, chatId, sender, avatarUrl });
    },
    setBadgeCount: (count) => {
      const n = Number.isFinite(Number(count)) ? Number(count) : 0;
      sendToHost({ type: 'set-badge-count', count: n });
    },
    focusMainWindow: () => {
      sendToHost({ type: 'focus-main-window' });
    },
    dismissNotifications: (opts) => {
      const safe = opts && typeof opts === 'object' ? opts : {};
      const chatId = typeof safe.chatId === 'string' ? safe.chatId : '';
      sendToHost({ type: 'dismiss-notifications', chatId });
    },
  };
})();
""";

    public MainWindow()
    {
        InitializeComponent();

        Title = WrapperSettings.AppName;
        _notificationManager = new DesktopNotificationManager(OnDesktopNotificationClicked);
        _trayIcon = CreateTrayIcon();

        Loaded += MainWindow_Loaded;
        SourceInitialized += MainWindow_SourceInitialized;
        Closing += MainWindow_Closing;
        StateChanged += MainWindow_StateChanged;
        PreviewKeyDown += MainWindow_PreviewKeyDown;
    }

    private Forms.NotifyIcon CreateTrayIcon()
    {
        var icon = Drawing.SystemIcons.Application;
        if (!string.IsNullOrWhiteSpace(Environment.ProcessPath))
        {
            var extracted = Drawing.Icon.ExtractAssociatedIcon(Environment.ProcessPath);
            if (extracted is not null)
            {
                icon = extracted;
            }
        }

        var trayIcon = new Forms.NotifyIcon
        {
            Icon = icon,
            Visible = true,
            Text = WrapperSettings.AppName,
        };

        var menu = new Forms.ContextMenuStrip();
        menu.Items.Add("Open", null, (_, _) => RestoreFromTrayAndActivate());
        menu.Items.Add("Exit", null, (_, _) => ExitApplication());
        trayIcon.ContextMenuStrip = menu;
        trayIcon.MouseClick += (_, e) =>
        {
            if (e.Button == Forms.MouseButtons.Left)
            {
                RestoreFromTrayAndActivate();
            }
        };
        trayIcon.DoubleClick += (_, _) => RestoreFromTrayAndActivate();

        return trayIcon;
    }

    private void MainWindow_SourceInitialized(object? sender, EventArgs e)
    {
        if (PresentationSource.FromVisual(this) is HwndSource source)
        {
            source.AddHook(WndProc);
            try
            {
                NativeMethods.ChangeWindowMessageFilterEx(
                    source.Handle,
                    App.ShowExistingWindowSignal,
                    NativeMethods.MsgfltAllow,
                    IntPtr.Zero);
            }
            catch
            {
                // Ignore on unsupported systems.
            }
            ApplyDarkTitleBar(source.Handle);
        }
    }

    private static void ApplyDarkTitleBar(IntPtr hwnd)
    {
        if (hwnd == IntPtr.Zero)
        {
            return;
        }

        try
        {
            var enabled = 1;
            NativeMethods.DwmSetWindowAttribute(
                hwnd,
                NativeMethods.DwmwaUseImmersiveDarkMode,
                ref enabled,
                sizeof(int));

            // COLORREF format: 0x00BBGGRR
            var captionColor = 0x000000;
            NativeMethods.DwmSetWindowAttribute(
                hwnd,
                NativeMethods.DwmwaCaptionColor,
                ref captionColor,
                sizeof(int));

            var textColor = 0x00FFFFFF;
            NativeMethods.DwmSetWindowAttribute(
                hwnd,
                NativeMethods.DwmwaTextColor,
                ref textColor,
                sizeof(int));
        }
        catch
        {
            // Ignore on unsupported Windows versions.
        }
    }

    private IntPtr WndProc(IntPtr hwnd, int msg, IntPtr wParam, IntPtr lParam, ref bool handled)
    {
        if ((uint)msg == App.ShowExistingWindowSignal)
        {
            RestoreFromTrayAndActivate();
            handled = true;
        }

        return IntPtr.Zero;
    }

    private async void MainWindow_Loaded(object sender, RoutedEventArgs e)
    {
        await InitializeBrowserAsync();
    }

    private async Task InitializeBrowserAsync()
    {
        ShowLoading("Starting browser engine...");
        HideError();

        try
        {
            var userDataFolder = BuildUserDataFolder();
            Directory.CreateDirectory(userDataFolder);

            await EnsureWebViewInitializedAsync(userDataFolder);
            await ConfigureWebViewAsync();
            NavigateToStartUrl();
        }
        catch (WebView2RuntimeNotFoundException)
        {
            HideLoading();
            ShowError(
                "WebView2 Runtime Not Found",
                "Install Microsoft Edge WebView2 Runtime and start the app again."
            );
        }
        catch (Exception ex)
        {
            HideLoading();
            ShowError("Failed to initialize WebView2", ex.Message);
        }
    }

    private async Task EnsureWebViewInitializedAsync(string userDataFolder)
    {
        try
        {
            await InitializeWebViewEnvironmentAsync(userDataFolder);
        }
        catch (Exception) when (TryResetUserDataFolder(userDataFolder))
        {
            await InitializeWebViewEnvironmentAsync(userDataFolder);
        }
    }

    private async Task InitializeWebViewEnvironmentAsync(string userDataFolder)
    {
        var environment = await CoreWebView2Environment.CreateAsync(userDataFolder: userDataFolder);
        await Browser.EnsureCoreWebView2Async(environment);
    }

    private async Task ConfigureWebViewAsync()
    {
        var core = Browser.CoreWebView2;
        if (core is null)
        {
            return;
        }

        if (_webBridgeInitialized)
        {
            return;
        }

        core.Settings.IsStatusBarEnabled = false;
        core.Settings.AreBrowserAcceleratorKeysEnabled = true;
        core.Settings.AreDefaultContextMenusEnabled = true;
        core.Settings.AreDefaultScriptDialogsEnabled = true;

        core.NavigationStarting += Core_NavigationStarting;
        core.NavigationCompleted += Core_NavigationCompleted;
        core.ProcessFailed += Core_ProcessFailed;
        core.PermissionRequested += Core_PermissionRequested;
        core.WebMessageReceived += Core_WebMessageReceived;

        await core.AddScriptToExecuteOnDocumentCreatedAsync(DesktopBridgeScript);
        _webBridgeInitialized = true;
    }

    private static bool TryResetUserDataFolder(string userDataFolder)
    {
        try
        {
            if (Directory.Exists(userDataFolder))
            {
                Directory.Delete(userDataFolder, recursive: true);
            }

            Directory.CreateDirectory(userDataFolder);
            return true;
        }
        catch
        {
            return false;
        }
    }

    private void Core_PermissionRequested(object? sender, CoreWebView2PermissionRequestedEventArgs e)
    {
        if (e.PermissionKind == CoreWebView2PermissionKind.Notifications)
        {
            e.State = CoreWebView2PermissionState.Allow;
            e.Handled = true;
        }
    }

    private void Core_WebMessageReceived(object? sender, CoreWebView2WebMessageReceivedEventArgs e)
    {
        DesktopBridgeMessage? message;
        try
        {
            message = JsonSerializer.Deserialize<DesktopBridgeMessage>(e.WebMessageAsJson, BridgeJsonOptions);
        }
        catch
        {
            return;
        }

        if (message is null || string.IsNullOrWhiteSpace(message.Type))
        {
            return;
        }

        switch (message.Type)
        {
            case "show-notification":
            {
                var title = string.IsNullOrWhiteSpace(message.Title) ? WrapperSettings.AppName : message.Title.Trim();
                var body = string.IsNullOrWhiteSpace(message.Body) ? "New message" : message.Body.Trim();
                var senderName = string.IsNullOrWhiteSpace(message.Sender) ? null : message.Sender.Trim();
                var avatarUrl = string.IsNullOrWhiteSpace(message.AvatarUrl) ? null : message.AvatarUrl.Trim();
                _notificationManager.Enqueue(new DesktopNotificationPayload(title, body, message.ChatId, senderName, avatarUrl));
                break;
            }
            case "set-badge-count":
                UpdateTrayBadge(Math.Max(0, message.Count));
                break;
            case "focus-main-window":
                RestoreFromTrayAndActivate();
                break;
            case "dismiss-notifications":
                if (string.IsNullOrWhiteSpace(message.ChatId))
                {
                    _notificationManager.DismissAll();
                }
                else
                {
                    _notificationManager.DismissByChatId(message.ChatId);
                }
                break;
        }
    }

    private void UpdateTrayBadge(int count)
    {
        var text = count > 0 ? $"{WrapperSettings.AppName} ({count})" : WrapperSettings.AppName;
        _trayIcon.Text = text.Length <= 63 ? text : text[..63];
    }

    private void OnDesktopNotificationClicked(DesktopNotificationPayload payload)
    {
        if (!Dispatcher.CheckAccess())
        {
            Dispatcher.Invoke(() => OnDesktopNotificationClicked(payload));
            return;
        }

        if (!string.IsNullOrWhiteSpace(payload.ChatId))
        {
            _notificationManager.DismissByChatId(payload.ChatId);
        }

        RestoreFromTrayAndActivate();
        _ = OpenChatFromNotificationAsync(payload);
    }

    private async Task OpenChatFromNotificationAsync(DesktopNotificationPayload payload)
    {
        var core = Browser.CoreWebView2;
        if (core is null)
        {
            return;
        }

        var chatIdJson = JsonSerializer.Serialize(payload.ChatId);
        var titleJson = JsonSerializer.Serialize(payload.Title);

        var scriptTemplate = @"
(() => {
  const detail = { chatId: __CHAT_ID__, title: __TITLE__ };
  window.dispatchEvent(new CustomEvent('pulse-desktop-open-chat', { detail }));

  const normalize = (v) => String(v || '').replace(/\s+/g, ' ').trim().toLowerCase();

  if (detail.chatId) {
    const chatById = Array.from(document.querySelectorAll('[data-chat-id]'))
      .find((el) => el.getAttribute('data-chat-id') === detail.chatId);
    if (chatById) {
      chatById.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true }));
      return true;
    }
  }

  const wanted = normalize(detail.title);
  if (!wanted) return false;

  const candidates = Array.from(document.querySelectorAll('button,[role=""button""],a,div'));
  const matched = candidates.find((el) => {
    const text = normalize(el.textContent || '');
    return text === wanted || text.startsWith(wanted + ' ');
  });

  if (matched) {
    matched.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true }));
    return true;
  }

  return false;
})();
";
        var script = scriptTemplate
            .Replace("__CHAT_ID__", chatIdJson)
            .Replace("__TITLE__", titleJson);

        try
        {
            await core.ExecuteScriptAsync(script);
        }
        catch
        {
            // Ignore script execution failures; app remains usable.
        }
    }

    private void Core_NavigationStarting(object? sender, CoreWebView2NavigationStartingEventArgs e)
    {
        if (Uri.TryCreate(e.Uri, UriKind.Absolute, out var uri))
        {
            _lastAttemptedUri = uri;
        }

        ShowLoading("Loading page...");
    }

    private void Core_NavigationCompleted(object? sender, CoreWebView2NavigationCompletedEventArgs e)
    {
        HideLoading();

        if (e.IsSuccess)
        {
            HideError();
            return;
        }

        ShowError(
            "Network or navigation error",
            $"WebView2 error code: {e.WebErrorStatus}. Check connectivity and server availability."
        );
    }

    private void Core_ProcessFailed(object? sender, CoreWebView2ProcessFailedEventArgs e)
    {
        ShowError(
            "WebView2 process crashed",
            $"Reason: {e.ProcessFailedKind}. Click Retry to reload."
        );
    }

    private void MainWindow_StateChanged(object? sender, EventArgs e)
    {
        if (_isHidingToTray)
        {
            return;
        }

        if (WindowState == WindowState.Minimized)
        {
            HideToTray();
        }
    }

    private void MainWindow_Closing(object? sender, System.ComponentModel.CancelEventArgs e)
    {
        if (!_exitRequested)
        {
            e.Cancel = true;
            HideToTray();
            return;
        }

        _trayIcon.Visible = false;
        _trayIcon.Dispose();
        _notificationManager.Dispose();
    }

    private void ExitApplication()
    {
        _exitRequested = true;
        Close();
        System.Windows.Application.Current.Shutdown();
    }

    private void HideToTray()
    {
        if (_isHidingToTray)
        {
            return;
        }
        _isHidingToTray = true;
        try
        {
            if (WindowState != WindowState.Minimized)
            {
                _windowStateBeforeTray = WindowState;
            }
            else if (_windowStateBeforeTray is not WindowState.Normal and not WindowState.Maximized)
            {
                _windowStateBeforeTray = WindowState.Maximized;
            }

            _restoreBoundsBeforeTray = _windowStateBeforeTray == WindowState.Normal
                ? RestoreBounds
                : new Rect(Left, Top, Width, Height);

            ShowInTaskbar = false;
            Hide();
            _isParkedInTray = true;
        }
        finally
        {
            _isHidingToTray = false;
        }
    }

    private void RestoreFromTrayAndActivate()
    {
        if (!Dispatcher.CheckAccess())
        {
            Dispatcher.Invoke(RestoreFromTrayAndActivate);
            return;
        }

        var shouldUnpark = _isParkedInTray || !ShowInTaskbar || IsWindowOffScreen();
        ShowInTaskbar = true;
        if (!IsVisible)
        {
            Show();
        }

        if (shouldUnpark)
        {
            if (_windowStateBeforeTray == WindowState.Normal)
            {
                WindowState = WindowState.Normal;
                var bounds = GetSafeRestoreBounds();
                Left = bounds.Left;
                Top = bounds.Top;
                Width = bounds.Width;
                Height = bounds.Height;
            }
            else
            {
                WindowState = WindowState.Maximized;
            }

            _isParkedInTray = false;
        }

        BringWindowToFront();

        _ = EnsureWebViewHealthyAfterRestoreAsync();
    }

    internal void RestoreFromExternalActivation()
    {
        RestoreFromTrayAndActivate();
    }

    private bool IsWindowOffScreen()
    {
        if (WindowState == WindowState.Maximized)
        {
            return false;
        }

        var windowRect = new Rect(Left, Top, Math.Max(Width, 120), Math.Max(Height, 120));
        foreach (var screen in Forms.Screen.AllScreens)
        {
            var area = screen.WorkingArea;
            var screenRect = new Rect(area.Left, area.Top, area.Width, area.Height);
            if (windowRect.IntersectsWith(screenRect))
            {
                return false;
            }
        }

        return true;
    }

    private Rect GetSafeRestoreBounds()
    {
        var bounds = _restoreBoundsBeforeTray;
        if (bounds.Width < 260 || bounds.Height < 180)
        {
            bounds = new Rect(100, 100, 1280, 820);
        }

        if (!IsRectVisible(bounds))
        {
            var primary = Forms.Screen.PrimaryScreen?.WorkingArea
                          ?? new Drawing.Rectangle(0, 0, 1920, 1080);
            var width = Math.Min(Math.Max(bounds.Width, 900), primary.Width);
            var height = Math.Min(Math.Max(bounds.Height, 640), primary.Height);
            var left = primary.Left + Math.Max(0, (primary.Width - width) / 2.0);
            var top = primary.Top + Math.Max(0, (primary.Height - height) / 2.0);
            bounds = new Rect(left, top, width, height);
        }

        return bounds;
    }

    private static bool IsRectVisible(Rect rect)
    {
        foreach (var screen in Forms.Screen.AllScreens)
        {
            var area = screen.WorkingArea;
            var screenRect = new Rect(area.Left, area.Top, area.Width, area.Height);
            if (rect.IntersectsWith(screenRect))
            {
                return true;
            }
        }

        return false;
    }

    private void BringWindowToFront()
    {
        Activate();
        Topmost = true;
        Topmost = false;
        Focus();

        var hwnd = new WindowInteropHelper(this).Handle;
        if (hwnd == IntPtr.Zero)
        {
            return;
        }

        NativeMethods.ShowWindow(hwnd, NativeMethods.SwShow);
        NativeMethods.ShowWindow(hwnd, NativeMethods.SwRestore);
        NativeMethods.SetForegroundWindow(hwnd);
    }

    private async Task EnsureWebViewHealthyAfterRestoreAsync()
    {
        if (_restoringWebView)
        {
            return;
        }

        _restoringWebView = true;
        try
        {
            if (Browser.CoreWebView2 is null)
            {
                await InitializeBrowserAsync();
                return;
            }

            try
            {
                var healthProbe = Browser.CoreWebView2.ExecuteScriptAsync("document.readyState");
                var completed = await Task.WhenAny(healthProbe, Task.Delay(2000));
                if (completed != healthProbe)
                {
                    NavigateToStartUrl();
                    return;
                }

                await healthProbe;
            }
            catch
            {
                NavigateToStartUrl();
                return;
            }
        }
        finally
        {
            _restoringWebView = false;
        }
    }

    private void MainWindow_PreviewKeyDown(object sender, System.Windows.Input.KeyEventArgs e)
    {
        if (e.Key == Key.R && Keyboard.Modifiers.HasFlag(ModifierKeys.Control))
        {
            e.Handled = true;
            ReloadPage();
            return;
        }

        if (e.Key == Key.F11)
        {
            e.Handled = true;
            ToggleFullscreen();
            return;
        }

        if (e.Key == Key.Escape && _isFullScreen)
        {
            e.Handled = true;
            ToggleFullscreen();
        }
    }

    private void RetryButton_Click(object sender, RoutedEventArgs e)
    {
        if (Browser.CoreWebView2 is null)
        {
            _ = InitializeBrowserAsync();
            return;
        }

        if (_lastAttemptedUri is not null)
        {
            Browser.CoreWebView2.Navigate(_lastAttemptedUri.ToString());
            return;
        }

        NavigateToStartUrl();
    }

    private void OpenStartUrlButton_Click(object sender, RoutedEventArgs e)
    {
        NavigateToStartUrl();
    }

    private void NavigateToStartUrl()
    {
        if (Browser.CoreWebView2 is null)
        {
            return;
        }

        if (!Uri.TryCreate(WrapperSettings.StartUrl, UriKind.Absolute, out var url))
        {
            HideLoading();
            ShowError("Invalid start URL", $"Check WrapperStartUrl: {WrapperSettings.StartUrl}");
            return;
        }

        _lastAttemptedUri = url;
        Browser.CoreWebView2.Navigate(url.ToString());
    }

    private void ReloadPage()
    {
        if (Browser.CoreWebView2 is null)
        {
            return;
        }

        ShowLoading("Reloading page...");
        Browser.CoreWebView2.Reload();
    }

    private void ToggleFullscreen()
    {
        if (!_isFullScreen)
        {
            _savedWindowStyle = WindowStyle;
            _savedResizeMode = ResizeMode;
            _savedWindowState = WindowState;
            _savedRestoreBounds = RestoreBounds;

            WindowStyle = WindowStyle.None;
            ResizeMode = ResizeMode.NoResize;
            WindowState = WindowState.Maximized;
            _isFullScreen = true;
            return;
        }

        WindowStyle = _savedWindowStyle;
        ResizeMode = _savedResizeMode;
        WindowState = _savedWindowState;

        if (_savedWindowState == WindowState.Normal)
        {
            Left = _savedRestoreBounds.Left;
            Top = _savedRestoreBounds.Top;
            Width = _savedRestoreBounds.Width;
            Height = _savedRestoreBounds.Height;
        }

        _isFullScreen = false;
    }

    private static string BuildUserDataFolder()
    {
        var company = SafePathPart(WrapperSettings.CompanyName);
        var app = SafePathPart(WrapperSettings.AppName);
        return Path.Combine(
            Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),
            company,
            app,
            "WebView2"
        );
    }

    private static string SafePathPart(string value)
    {
        var safe = value.Trim();
        foreach (var c in Path.GetInvalidFileNameChars())
        {
            safe = safe.Replace(c, '_');
        }

        return string.IsNullOrWhiteSpace(safe) ? "PulseDesktop" : safe;
    }

    private void ShowLoading(string message)
    {
        LoadingText.Text = message;
        LoadingOverlay.Visibility = Visibility.Visible;
    }

    private void HideLoading()
    {
        LoadingOverlay.Visibility = Visibility.Collapsed;
    }

    private void ShowError(string title, string details)
    {
        ErrorTitle.Text = title;
        ErrorDetails.Text = details;
        ErrorOverlay.Visibility = Visibility.Visible;
    }

    private void HideError()
    {
        ErrorOverlay.Visibility = Visibility.Collapsed;
    }

    private sealed class DesktopBridgeMessage
    {
        public string? Type { get; init; }
        public string? Title { get; init; }
        public string? Body { get; init; }
        public string? ChatId { get; init; }
        public string? Sender { get; init; }
        public string? AvatarUrl { get; init; }
        public int Count { get; init; }
    }
}
