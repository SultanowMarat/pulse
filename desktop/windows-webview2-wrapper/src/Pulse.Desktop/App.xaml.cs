using System;
using System.Threading;
using System.Windows;
using WpfApplication = System.Windows.Application;

namespace Pulse.Desktop;

public partial class App : WpfApplication
{
    private const string SingleInstanceMutexName = @"Local\PulseDesktop.SingleInstance";
    private const string ActivateEventName = @"Local\PulseDesktop.Activate";
    private static readonly uint ShowExistingWindowMessage = NativeMethods.RegisterWindowMessage("PulseDesktop.ShowExistingWindow");
    private Mutex? _singleInstanceMutex;
    private EventWaitHandle? _activateEvent;
    private RegisteredWaitHandle? _activateEventRegistration;
    private volatile bool _isExiting;
    private const int ParkedCoordinateThreshold = -30000;

    internal static uint ShowExistingWindowSignal => ShowExistingWindowMessage;

    protected override void OnStartup(StartupEventArgs e)
    {
        var createdNew = false;
        _singleInstanceMutex = new Mutex(initiallyOwned: true, SingleInstanceMutexName, out createdNew);

        if (!createdNew)
        {
            SignalExistingInstance();
            Shutdown();
            return;
        }

        EnsureActivationSignal();
        ShutdownMode = ShutdownMode.OnExplicitShutdown;

        if (AutoUpdater.TryApplyUpdateOnStartup())
        {
            Shutdown();
            return;
        }

        var mainWindow = new MainWindow();
        MainWindow = mainWindow;
        mainWindow.Show();

        base.OnStartup(e);
    }

    protected override void OnExit(ExitEventArgs e)
    {
        _isExiting = true;

        if (_activateEventRegistration is not null)
        {
            _activateEventRegistration.Unregister(null);
            _activateEventRegistration = null;
        }

        _activateEvent?.Dispose();
        _activateEvent = null;

        if (_singleInstanceMutex is not null)
        {
            try
            {
                _singleInstanceMutex.ReleaseMutex();
            }
            catch (ApplicationException)
            {
                // Ignore when mutex ownership was already released.
            }

            _singleInstanceMutex.Dispose();
            _singleInstanceMutex = null;
        }

        base.OnExit(e);
    }

    private void EnsureActivationSignal()
    {
        _activateEvent = new EventWaitHandle(false, EventResetMode.AutoReset, ActivateEventName);
        _activateEventRegistration = ThreadPool.RegisterWaitForSingleObject(
            _activateEvent,
            (_, timedOut) =>
            {
                if (timedOut || _isExiting)
                {
                    return;
                }

                Dispatcher.BeginInvoke(new Action(() =>
                {
                    if (_isExiting)
                    {
                        return;
                    }

                    if (MainWindow is MainWindow window)
                    {
                        window.RestoreFromExternalActivation();
                        return;
                    }

                    MainWindow?.Activate();
                }));
            },
            null,
            Timeout.Infinite,
            false);
    }

    private static void SignalExistingInstance()
    {
        for (var attempt = 0; attempt < 5; attempt++)
        {
            try
            {
                using var activateEvent = EventWaitHandle.OpenExisting(ActivateEventName);
                activateEvent.Set();
            }
            catch
            {
                // Ignore and fallback to Win32 signaling.
            }

            TryActivateExistingWindow();
            if (attempt < 4)
            {
                Thread.Sleep(80);
            }
        }

        NativeMethods.PostMessage(NativeMethods.HwndBroadcast, ShowExistingWindowMessage, IntPtr.Zero, IntPtr.Zero);
    }

    private static void TryActivateExistingWindow()
    {
        try
        {
            var hwnd = NativeMethods.FindWindow(null, WrapperSettings.AppName);
            if (hwnd == IntPtr.Zero)
            {
                return;
            }

            NativeMethods.PostMessage(hwnd, ShowExistingWindowMessage, IntPtr.Zero, IntPtr.Zero);
            NativeMethods.ShowWindow(hwnd, NativeMethods.SwShow);
            NativeMethods.ShowWindow(hwnd, NativeMethods.SwRestore);
            if (NativeMethods.GetWindowRect(hwnd, out var rect)
                && (rect.Left <= ParkedCoordinateThreshold
                    || rect.Top <= ParkedCoordinateThreshold
                    || rect.Right <= ParkedCoordinateThreshold
                    || rect.Bottom <= ParkedCoordinateThreshold))
            {
                NativeMethods.SetWindowPos(
                    hwnd,
                    IntPtr.Zero,
                    120,
                    120,
                    0,
                    0,
                    NativeMethods.SwpNosize | NativeMethods.SwpNozorder | NativeMethods.SwpNoactivate);
            }

            NativeMethods.SetForegroundWindow(hwnd);
        }
        catch
        {
            // Best-effort fallback only.
        }
    }
}
