package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
)

type TokenResponse struct {
	AccessToken string `json:"access_token"`
}

func main() {
	var mw *walk.MainWindow
	var oldID, oldSecret, instID, newID, newSecret *walk.LineEdit
	var logBox *walk.TextEdit
	var btnRun *walk.PushButton

	// Функция для безопасного вывода логов из фонового потока в интерфейс
	logMsg := func(msg string) {
		mw.Synchronize(func() {
			logBox.AppendText(msg + "\r\n")
		})
	}

	MainWindow{
		AssignTo: &mw,
		Title:    "Перенос подложек Sentinel Hub",
		MinSize:  Size{Width: 500, Height: 550},
		Layout:   VBox{},
		Children: []Widget{
			GroupBox{
				Title:  "СТАРЫЙ АККАУНТ (Откуда забираем)",
				Layout: Grid{Columns: 2},
				Children: []Widget{
					Label{Text: "Старый Client ID:"},
					LineEdit{AssignTo: &oldID},
					Label{Text: "Старый Client Secret:"},
					LineEdit{AssignTo: &oldSecret, PasswordMode: true}, // PasswordMode скроет ввод звездочками
					Label{Text: "ID Конфигурации:"},
					LineEdit{AssignTo: &instID, Text: "464a9446-179b-4aa1-bc94-a36c79f47f6a"},
				},
			},
			GroupBox{
				Title:  "НОВЫЙ АККАУНТ (Куда переносим)",
				Layout: Grid{Columns: 2},
				Children: []Widget{
					Label{Text: "Новый Client ID:"},
					LineEdit{AssignTo: &newID},
					Label{Text: "Новый Client Secret:"},
					LineEdit{AssignTo: &newSecret, PasswordMode: true},
				},
			},
			Label{Text: "Журнал событий:"},
			TextEdit{
				AssignTo: &logBox,
				ReadOnly: true,
				VScroll:  true,
			},
			PushButton{
				AssignTo: &btnRun,
				Text:     "Запустить перенос",
				OnClicked: func() {
					// Проверка на пустые поля
					if oldID.Text() == "" || oldSecret.Text() == "" || newID.Text() == "" || newSecret.Text() == "" || instID.Text() == "" {
						walk.MsgBox(mw, "Ошибка", "Заполните все поля!", walk.MsgBoxIconWarning)
						return
					}

					btnRun.SetEnabled(false)
					logBox.SetText("")

					// Запускаем процесс в фоне (Горутина)
					go func() {
						// Обязательно возвращаем кнопку в рабочее состояние после завершения
						defer mw.Synchronize(func() { btnRun.SetEnabled(true) })

						logMsg("🔑 Подключение к СТАРОМУ аккаунту...")
						tokenOld, err := getToken(oldID.Text(), oldSecret.Text())
						if err != nil {
							logMsg(fmt.Sprintf("❌ Ошибка: %v", err))
							return
						}
						logMsg("✅ Успешно!")

						logMsg("🔑 Подключение к НОВОМУ аккаунту...")
						tokenNew, err := getToken(newID.Text(), newSecret.Text())
						if err != nil {
							logMsg(fmt.Sprintf("❌ Ошибка: %v", err))
							return
						}
						logMsg("✅ Успешно!")

						logMsg(fmt.Sprintf("📥 Чтение слоев из %s...", instID.Text()))
						oldLayers, err := getLayers(tokenOld, instID.Text())
						if err != nil {
							logMsg(fmt.Sprintf("❌ Ошибка: %v", err))
							return
						}

						logMsg("📦 Создание новой конфигурации...")
						newInstID, err := createInstance(tokenNew, "GP WMS Services (Migrated)")
						if err != nil {
							logMsg(fmt.Sprintf("❌ Ошибка: %v", err))
							return
						}

						logMsg(fmt.Sprintf("🔄 Перенос слоев (%d шт.)...", len(oldLayers)))
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
								logMsg(fmt.Sprintf("   ❌ Ошибка слоя '%s': %v", layerID, err))
							} else {
								logMsg(fmt.Sprintf("   ✅ Слой '%s' добавлен.", layerID))
							}
						}

						logMsg(fmt.Sprintf("\n🎉 ГОТОВО!\nНовый рабочий ID: %s", newInstID))
					}()
				},
			},
		},
	}.Run()
}

// ---- АПИ ФУНКЦИИ ОСТАЮТСЯ БЕЗ ИЗМЕНЕНИЙ ---- //

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
		return "", fmt.Errorf("неверный ID или Secret")
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
		return nil, fmt.Errorf("ошибка получения слоев: %d", resp.StatusCode)
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
		return "", fmt.Errorf("ошибка: %s", string(body))
	}

	body, _ := io.ReadAll(resp.Body)
	var res map[string]interface{}
	json.Unmarshal(body, &res)

	if id, ok := res["id"].(string); ok {
		return id, nil
	}
	return "", fmt.Errorf("id не найден в ответе")
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
