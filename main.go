package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

type TokenResponse struct {
	AccessToken string `json:"access_token"`
}

func main() {
	a := app.New()
	w := a.NewWindow("Sentinel Hub Migration Tool")
	w.Resize(fyne.NewSize(500, 600))

	oldClientID := widget.NewEntry()
	oldClientID.SetPlaceHolder("Old Client ID")
	oldClientSecret := widget.NewEntry()
	oldClientSecret.SetPlaceHolder("Old Client Secret")
	instanceID := widget.NewEntry()
	instanceID.SetText("464a9446-179b-4aa1-bc94-a36c79f47f6a")

	oldGroup := container.NewVBox(
		widget.NewLabelWithStyle("OLD ACCOUNT", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel("Client ID:"), oldClientID,
		widget.NewLabel("Client Secret:"), oldClientSecret,
		widget.NewLabel("Configuration ID:"), instanceID,
	)

	newClientID := widget.NewEntry()
	newClientID.SetPlaceHolder("New Client ID")
	newClientSecret := widget.NewEntry()
	newClientSecret.SetPlaceHolder("New Client Secret")

	newGroup := container.NewVBox(
		widget.NewLabelWithStyle("NEW ACCOUNT", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel("Client ID:"), newClientID,
		widget.NewLabel("Client Secret:"), newClientSecret,
	)

	logBox := widget.NewMultiLineEntry()
	logBox.Disable()
	
	logMsg := func(text string) {
		logBox.SetText(logBox.Text + text + "\n")
		logBox.CursorRow = len(strings.Split(logBox.Text, "\n"))
	}

	var btnRun *widget.Button
	btnRun = widget.NewButton("Run Migration", func() {
		if oldClientID.Text == "" || oldClientSecret.Text == "" || newClientID.Text == "" || newClientSecret.Text == "" || instanceID.Text == "" {
			dialog.ShowError(fmt.Errorf("fill all fields"), w)
			return
		}

		btnRun.Disable()
		logBox.SetText("")

		go func() {
			defer btnRun.Enable()

			logMsg("Connecting to OLD account...")
			tokenOld, err := getToken(oldClientID.Text, oldClientSecret.Text)
			if err != nil {
				logMsg(fmt.Sprintf("Error: %v", err))
				return
			}
			logMsg("Success")

			logMsg("Connecting to NEW account...")
			tokenNew, err := getToken(newClientID.Text, newClientSecret.Text)
			if err != nil {
				logMsg(fmt.Sprintf("Error: %v", err))
				return
			}
			logMsg("Success")

			logMsg(fmt.Sprintf("Reading layers from %s...", instanceID.Text))
			oldLayers, err := getLayers(tokenOld, instanceID.Text)
			if err != nil {
				logMsg(fmt.Sprintf("Error: %v", err))
				return
			}

			logMsg("Creating new instance...")
			newInstID, err := createInstance(tokenNew, "GP WMS Services (Migrated)")
			if err != nil {
				logMsg(fmt.Sprintf("Error: %v", err))
				return
			}

			logMsg(fmt.Sprintf("Migrating layers (%d)...", len(oldLayers)))
			for _, layer := range oldLayers {
				layerID := "Unknown"
				if id, ok := layer["id"].(string); ok {
					layerID = id
				}

				delete(layer, "instanceId")
				delete(layer, "lastUpdated")
				delete(layer, "created")

				err := postLayer(tokenNew, newInstID, layer)
				if err != nil {
					logMsg(fmt.Sprintf("  Layer '%s' error: %v", layerID, err))
				} else {
					logMsg(fmt.Sprintf("  Layer '%s' added", layerID))
				}
			}

			logMsg(fmt.Sprintf("\nDONE!\nNew Instance ID: %s", newInstID))
		}()
	})
	btnRun.Importance = widget.HighImportance

	content := container.NewVBox(
		oldGroup,
		widget.NewSeparator(),
		newGroup,
		widget.NewSeparator(),
		widget.NewLabel("Log:"),
	)

	split := container.NewVSplit(content, logBox)
	split.SetOffset(0.6)

	mainLayout := container.NewBorder(nil, btnRun, nil, nil, split)

	w.SetContent(mainLayout)
	w.ShowAndRun()
}

func getToken(clientID, clientSecret string) (string, error) {
	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)

	req, err := http.NewRequest("POST", "https://services.sentinel-hub.com/oauth/token", strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		return "", fmt.Errorf("401 unauthorized")
	}
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("http error: %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var tr TokenResponse
	json.Unmarshal(body, &tr)
	return tr.AccessToken, nil
}

func getLayers(token, instanceID string) ([]map[string]interface{}, error) {
	url := fmt.Sprintf("https://services.sentinel-hub.com/configuration/v1/wms/instances/%s/layers", instanceID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to get layers, status: %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var layers []map[string]interface{}
	json.Unmarshal(body, &layers)
	return layers, nil
}

func createInstance(token, name string) (string, error) {
	payload := map[string]string{"name": name}
	jsonPayload, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", "https://services.sentinel-hub.com/configuration/v1/wms/instances", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return "", err
	}
	req.Header.Add("Authorization", "Bearer "+token)
	req.Header.Add("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to create instance: %s", string(body))
	}

	body, _ := io.ReadAll(resp.Body)
	var res map[string]interface{}
	json.Unmarshal(body, &res)

	if id, ok := res["id"].(string); ok {
		return id, nil
	}
	return "", fmt.Errorf("id not found in response")
}

func postLayer(token, instanceID string, layer map[string]interface{}) error {
	url := fmt.Sprintf("https://services.sentinel-hub.com/configuration/v1/wms/instances/%s/layers", instanceID)
	jsonPayload, _ := json.Marshal(layer)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return err
	}
	req.Header.Add("Authorization", "Bearer "+token)
	req.Header.Add("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf(string(body))
	}
	return nil
}
