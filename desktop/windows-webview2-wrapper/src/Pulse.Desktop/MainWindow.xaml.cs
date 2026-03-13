using System;
using System.IO;
using System.Threading.Tasks;
using System.Windows;
using System.Windows.Input;
using Microsoft.Web.WebView2.Core;

namespace Pulse.Desktop;

public partial class MainWindow : Window
{
    private bool _isFullScreen;
    private WindowStyle _savedWindowStyle;
    private ResizeMode _savedResizeMode;
    private WindowState _savedWindowState;
    private Rect _savedRestoreBounds;
    private Uri? _lastAttemptedUri;

    public MainWindow()
    {
        InitializeComponent();
        Title = WrapperSettings.AppName;
        Loaded += MainWindow_Loaded;
        PreviewKeyDown += MainWindow_PreviewKeyDown;
    }

    private async void MainWindow_Loaded(object sender, RoutedEventArgs e)
    {
        await InitializeBrowserAsync();
    }

    private async Task InitializeBrowserAsync()
    {
        ShowLoading("Инициализация браузера...");
        HideError();

        try
        {
            var userDataFolder = BuildUserDataFolder();
            Directory.CreateDirectory(userDataFolder);

            var environment = await CoreWebView2Environment.CreateAsync(userDataFolder: userDataFolder);
            await Browser.EnsureCoreWebView2Async(environment);
            ConfigureWebView();
            NavigateToStartUrl();
        }
        catch (WebView2RuntimeNotFoundException)
        {
            HideLoading();
            ShowError(
                "Не найден WebView2 Runtime",
                "Установите Microsoft Edge WebView2 Runtime и запустите приложение снова."
            );
        }
        catch (Exception ex)
        {
            HideLoading();
            ShowError("Не удалось инициализировать WebView2", ex.Message);
        }
    }

    private void ConfigureWebView()
    {
        var core = Browser.CoreWebView2;
        if (core is null)
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
    }

    private void Core_NavigationStarting(object? sender, CoreWebView2NavigationStartingEventArgs e)
    {
        if (Uri.TryCreate(e.Uri, UriKind.Absolute, out var uri))
        {
            _lastAttemptedUri = uri;
        }

        ShowLoading("Загрузка страницы...");
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
            "Ошибка сети или навигации",
            $"Код ошибки WebView2: {e.WebErrorStatus}. Проверьте сеть и доступность сайта."
        );
    }

    private void Core_ProcessFailed(object? sender, CoreWebView2ProcessFailedEventArgs e)
    {
        ShowError(
            "Процесс WebView2 завершился с ошибкой",
            $"Причина: {e.ProcessFailedKind}. Нажмите \"Повторить\" для перезагрузки."
        );
    }

    private void MainWindow_PreviewKeyDown(object sender, KeyEventArgs e)
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
            ShowError("Неверный стартовый URL", $"Проверьте значение WrapperStartUrl: {WrapperSettings.StartUrl}");
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

        ShowLoading("Обновление страницы...");
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
}
