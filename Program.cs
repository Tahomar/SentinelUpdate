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
        Controls.Add(new Label { Text = "Client Secret:", Location = new Point(10, y + 3), Width = 100 });
        newSecret = new TextBox { Location = new Point(120, y), Width = 300 };
        Controls.Add(newSecret);

        y += 40;
        
        btnRun = new Button { Text = "Run Migration", Location = new Point(10, y), Width = 410, Height = 40, BackColor = Color.LightGreen, Font = new Font(Font, FontStyle.Bold) };
        btnRun.Click += BtnRun_Click;
        Controls.Add(btnRun);

        y += 50;
        
        Controls.Add(new Label { Text = "Log:", Location = new Point(10, y), AutoSize = true });
        y += 20;
        logBox = new RichTextBox { Location = new Point(10, y), Width = 410, Height = 200, ReadOnly = true, BackColor = Color.White };
        Controls.Add(logBox);
    }

    private void Log(string text)
    {
        if (InvokeRequired)
        {
            Invoke(new Action<string>(Log), text);
            return;
        }
        logBox.AppendText(text + Environment.NewLine);
        logBox.ScrollToCaret();
    }

    private async void BtnRun_Click(object? sender, EventArgs e)
    {
        if (string.IsNullOrWhiteSpace(oldClientId.Text) || string.IsNullOrWhiteSpace(oldSecret.Text) || 
            string.IsNullOrWhiteSpace(newClientId.Text) || string.IsNullOrWhiteSpace(newSecret.Text))
        {
            MessageBox.Show("Please fill all fields!", "Error", MessageBoxButtons.OK, MessageBoxIcon.Warning);
            return;
        }

        btnRun.Enabled = false;
        logBox.Clear();

        try
        {
            Log("Connecting to OLD account...");
            string tokenOld = await GetToken(oldClientId.Text, oldSecret.Text);
            Log("Success");

            Log("Connecting to NEW account...");
            string tokenNew = await GetToken(newClientId.Text, newSecret.Text);
            Log("Success");

            Log($"Reading layers from {instanceId.Text}...");
            var layers = await GetLayers(tokenOld, instanceId.Text);

            Log("Creating new instance...");
            string newInstId = await CreateInstance(tokenNew, "GP WMS Services (Migrated)");

            Log($"Migrating layers ({layers.Count})...");
            foreach (var layer in layers)
            {
                string layerId = layer["id"]?.ToString() ?? "Unknown";
                layer.AsObject().Remove("instanceId");
                layer.AsObject().Remove("lastUpdated");
                layer.AsObject().Remove("created");

                try
                {
                    await PostLayer(tokenNew, newInstId, layer);
                    Log($"  Layer '{layerId}' added");
                }
                catch (Exception ex)
                {
                    Log($"  Layer '{layerId}' error: {ex.Message}");
                }
            }

            Log("\nDONE!");
            Log($"New Instance ID: {newInstId}");
            MessageBox.Show("Migration Complete!", "Success", MessageBoxButtons.OK, MessageBoxIcon.Information);
        }
        catch (Exception ex)
        {
            Log($"CRITICAL ERROR: {ex.Message}");
        }
        finally
        {
            btnRun.Enabled = true;
        }
    }

    private async Task<string> GetToken(string clientId, string clientSecret)
    {
        var request = new HttpRequestMessage(HttpMethod.Post, "https://services.sentinel-hub.com/oauth/token");
        request.Content = new FormUrlEncodedContent(new[]
        {
            new KeyValuePair<string, string>("grant_type", "client_credentials"),
            new KeyValuePair<string, string>("client_id", clientId),
            new KeyValuePair<string, string>("client_secret", clientSecret)
        });

        var response = await client.SendAsync(request);
        response.EnsureSuccessStatusCode();
        var json = await response.Content.ReadAsStringAsync();
        var data = JsonNode.Parse(json);
        return data?["access_token"]?.ToString() ?? throw new Exception("No token received");
    }

    private async Task<JsonArray> GetLayers(string token, string instId)
    {
        var request = new HttpRequestMessage(HttpMethod.Get, $"https://services.sentinel-hub.com/configuration/v1/wms/instances/{instId}/layers");
        request.Headers.Authorization = new AuthenticationHeaderValue("Bearer", token);
        
        var response = await client.SendAsync(request);
        response.EnsureSuccessStatusCode();
        var json = await response.Content.ReadAsStringAsync();
        return JsonNode.Parse(json)?.AsArray() ?? new JsonArray();
    }

    private async Task<string> CreateInstance(string token, string name)
    {
        var request = new HttpRequestMessage(HttpMethod.Post, "https://services.sentinel-hub.com/configuration/v1/wms/instances");
        request.Headers.Authorization = new AuthenticationHeaderValue("Bearer", token);
        var payload = JsonSerializer.Serialize(new { name = name });
        request.Content = new StringContent(payload, Encoding.UTF8, "application/json");

        var response = await client.SendAsync(request);
        response.EnsureSuccessStatusCode();
        var json = await response.Content.ReadAsStringAsync();
        var data = JsonNode.Parse(json);
        return data?["id"]?.ToString() ?? throw new Exception("Instance ID not found");
    }

    private async Task PostLayer(string token, string instId, JsonNode layer)
    {
        var request = new HttpRequestMessage(HttpMethod.Post, $"https://services.sentinel-hub.com/configuration/v1/wms/instances/{instId}/layers");
        request.Headers.Authorization = new AuthenticationHeaderValue("Bearer", token);
        request.Content = new StringContent(layer.ToJsonString(), Encoding.UTF8, "application/json");

        var response = await client.SendAsync(request);
        if (!response.IsSuccessStatusCode)
        {
            string err = await response.Content.ReadAsStringAsync();
            throw new Exception(err);
        }
    }
}
