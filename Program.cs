using System.Net.Http.Headers;
using System.Text;
using System.Text.Json;
using System.Text.Json.Nodes;

namespace SentinelTool;

static class Program
{
    [STAThread]
    static void Main()
    {
        ApplicationConfiguration.Initialize();
        Application.Run(new MainForm());
    }
}

public class MainForm : Form
{
    TextBox oldClientId, oldSecret, instanceId;
    TextBox newClientId, newSecret;
    RichTextBox logBox;
    Button btnRun;
    static readonly HttpClient client = new HttpClient();

    public MainForm()
    {
        Text = "Sentinel Hub Migration Tool (C# Edition)";
        Size = new Size(450, 600);
        StartPosition = FormStartPosition.CenterScreen;
        FormBorderStyle = FormBorderStyle.FixedDialog;
        MaximizeBox = false;

        // Достаем иконку из самого exe файла и ставим в левый верхний угол
        try {
            Icon = Icon.ExtractAssociatedIcon(Application.ExecutablePath);
        } catch { }

        int y = 10;
        
        Controls.Add(new Label { Text = "OLD ACCOUNT", Font = new Font(Font, FontStyle.Bold), Location = new Point(10, y), AutoSize = true });
        y += 25;
        Controls.Add(new Label { Text = "Client ID:", Location = new Point(10, y + 3), Width = 100 });
        oldClientId = new TextBox { Location = new Point(120, y), Width = 300 };
        Controls.Add(oldClientId);
        
        y += 30;
        Controls.Add(new Label { Text = "Client Secret:", Location = new Point(10, y + 3), Width = 100 });
        oldSecret = new TextBox { Location = new Point(120, y), Width = 300 };
        Controls.Add(oldSecret);

        y += 30;
        Controls.Add(new Label { Text = "Instance ID:", Location = new Point(10, y + 3), Width = 100 });
        instanceId = new TextBox { Text = "464a9446-179b-4aa1-bc94-a36c79f47f6a", Location = new Point(120, y), Width = 300 };
        Controls.Add(instanceId);

        y += 40;
        
        Controls.Add(new Label { Text = "NEW ACCOUNT", Font = new Font(Font, FontStyle.Bold), Location = new Point(10, y), AutoSize = true });
        y += 25;
        Controls.Add(new Label { Text = "Client ID:", Location = new Point(10, y + 3), Width = 100 });
        newClientId = new TextBox { Location = new Point(120, y), Width = 300 };
        Controls.Add(newClientId);
        
        y += 30;
        Controls.Add(new Label { Text = "Client Secret:", Location
