// Waits for a click on a toast. Windows PowerShell can't subscribe to WinRT events itself,
// so toast.ps1 compiles this with the .NET Framework csc.exe on first use (see Get-ToastWaiter).
// Keep it C# 5: that is what csc.exe in the .NET Framework understands.

using System.Threading;
using Windows.UI.Notifications;

public sealed class ToastWaiter
{
    readonly ManualResetEventSlim done = new ManualResetEventSlim();

    // The arguments of what was clicked: "open" for the toast body, else the button's
    public string Action;
    // What was typed in the "reply" field
    public string Text;

    public ToastWaiter(ToastNotification toast)
    {
        toast.Activated += (sender, args) =>
        {
            var activated = args as ToastActivatedEventArgs;
            if (activated != null)
            {
                Action = activated.Arguments;
                object value;
                if (activated.UserInput != null && activated.UserInput.TryGetValue("reply", out value))
                    Text = value as string;
            }
            done.Set();
        };
        toast.Dismissed += (sender, args) =>
        {
            // Closed by the user. A banner that timed out is still in the notification center.
            if (args.Reason == ToastDismissalReason.UserCanceled)
                done.Set();
        };
    }

    public bool Wait(int milliseconds)
    {
        return done.Wait(milliseconds);
    }
}
