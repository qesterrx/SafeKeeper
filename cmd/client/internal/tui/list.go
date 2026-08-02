package tui

import (
	"context"
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/qesterrx/SafeKeeper/cmd/client/internal/model"
	"github.com/qesterrx/SafeKeeper/internal/logger"
	"github.com/rivo/tview"
)

type ListRepoService interface {
	AddItem(name string, host string, port string, login string, pswd string, register bool) error
	DelItem(name string) error
	GetItemsList() ([]*model.IfaceRepoItem, error)
	BackgroundProcess(ctx context.Context, interval time.Duration)
}

type ServerListForm struct {
	ta        *TUIApp
	table     *tview.Table
	statusBar *tview.TextView
	flex      *tview.Flex

	name string

	lr ListRepoService
}

func ConstructServerListForm(ta *TUIApp, lr ListRepoService) (*ServerListForm, error) {

	slf := ServerListForm{}

	slf.ta = ta
	slf.lr = lr
	slf.name = "ServerListForm"

	//Запускаем фоновые процессы сервисного слоя
	ta.wg.Go(func() {
		lr.BackgroundProcess(ta.ctx, 1000*time.Millisecond)
	})

	//Запускаем фоновый процесс обработки сигналов
	ta.wg.Go(func() {
		slf.RedrawLoop(ta.ctx, 3000*time.Millisecond)
	})

	// Создание таблицы
	slf.table = tview.NewTable()
	slf.table.SetBorders(true)
	slf.table.SetTitle(fmt.Sprintf("%s. Версия: %d", slf.ta.cnfig.ClientBuildInfo, slf.ta.cnfig.ClientVersion))
	slf.table.SetSelectable(true, false) // Выбор только строк
	slf.table.SetFixed(1, 0)             // Фиксируем заголовок

	// Статусная строка с подсказками
	slf.statusBar = tview.NewTextView().
		SetTextColor(tcell.ColorYellow).
		SetText(" [+]Добавить  [-]Удалить  [Enter]Открыть  [Esc]Выход")

	// Основной макет
	slf.flex = tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(slf.table, 0, 1, true).
		AddItem(slf.statusBar, 1, 0, false)

	slf.flex.SetBorder(true).SetTitle(fmt.Sprintf("%s :: %d  ", slf.ta.cnfig.ClientBuildInfo, slf.ta.cnfig.ClientVersion)).SetTitleAlign(tview.AlignCenter)

	// Обработка клавиш
	slf.flex.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case '+':
			slf.ShowSelectModeNewSrerverModal()
			return nil
		case '-':
			slf.AskDeleteServerModal()
			return nil
		}

		switch event.Key() {
		case tcell.KeyEscape:
			// Обработка Esc
			ta.StopTUIApp()
			return nil
		case tcell.KeyEnter:
			// Обработка Enter

			lri := slf.getSelectedRecord()
			if lri == nil {
				slf.ta.OpenErrorModal(fmt.Errorf("Нет выделенных записей"), slf.ShowServerListForm)
				return nil
			}

			ta.OpenPasswordModal(lri, slf.ShowServerListForm)

			return nil
		}
		return event
	})

	slf.ta.pages.AddPage(slf.name, slf.flex, true, true)
	return &slf, nil

}

func (slf *ServerListForm) Draw() {
	slf.table.Clear()

	// Заголовки
	headers := []string{"Наименование", "Сервер", "Порт", "Каталог", "Статус"}
	for col, header := range headers {
		cell := tview.NewTableCell(header).
			SetTextColor(tcell.ColorYellow).
			SetAlign(tview.AlignCenter).
			SetSelectable(false)
		slf.table.SetCell(0, col, cell)
	}

	//Данные
	data, err := slf.lr.GetItemsList()
	if err != nil {
		slf.ta.OpenErrorModal(err, slf.ShowServerListForm)
		return
	}

	for row, record := range data {

		slf.table.SetCell(row+1, 0, tview.NewTableCell(record.Name))
		slf.table.SetCell(row+1, 1, tview.NewTableCell(record.Host))
		slf.table.SetCell(row+1, 2, tview.NewTableCell(record.Port))
		slf.table.SetCell(row+1, 3, tview.NewTableCell(record.Path))
		slf.table.SetCell(row+1, 4, tview.NewTableCell(record.Status))

		slf.table.GetCell(row+1, 0).SetReference(record)

	}
}

func (slf *ServerListForm) getSelectedRecord() *model.IfaceRepoItem {
	row, col := slf.table.GetSelection()

	if row >= 0 && col >= 0 {
		cell := slf.table.GetCell(row, col)
		if cell != nil {
			ref := cell.GetReference()
			if ref != nil {
				if lri, ok := ref.(*model.IfaceRepoItem); ok {
					if lri != nil {
						return lri
					}

				}
			}

		}
	}
	return nil
}

// --------------------------------------------------------------------------------Новый объект
func (slf *ServerListForm) ShowSelectModeNewSrerverModal() {

	modal := tview.NewModal().
		SetText("Какое действие требуется выполнить").
		AddButtons([]string{"Регистрация", "Авторизация", "Отмена"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			if buttonLabel == "Регистрация" {
				slf.ShowNewServerModal(true)
				return
			} else if buttonLabel == "Авторизация" {
				slf.ShowNewServerModal(false)
				return
			}
			slf.ShowServerListForm()
		})

	slf.ta.pages.AddPage(slf.name+"_delete", modal, true, true)
	slf.ta.pages.SwitchToPage(slf.name + "_delete")
}

// --------------------------------------------------------------------------------Редактирование Логин/пароль
func (slf *ServerListForm) ShowNewServerModal(register bool) {
	form := tview.NewForm()

	form.AddInputField("Имя сервера: ", "", 20, nil, nil)
	form.AddInputField("Адрес сервера: ", "", 20, nil, nil)
	form.AddInputField("Порт: ", "32222", 20, nil, nil)
	form.AddInputField("Логин: ", "", 20, nil, nil)

	passwordInput := tview.NewInputField().
		SetLabel("Пароль: ").
		SetFieldWidth(30).
		SetMaskCharacter('*').
		SetAcceptanceFunc(tview.InputFieldMaxLength(30))
	form.AddFormItem(passwordInput)

	if register {
		passwordCheckInput := tview.NewInputField().
			SetLabel("Повторите пароль: ").
			SetFieldWidth(30).
			SetMaskCharacter('*').
			SetAcceptanceFunc(tview.InputFieldMaxLength(30))
		form.AddFormItem(passwordCheckInput)
	}

	form.AddButton("Применить", func() {
		name := form.GetFormItem(0).(*tview.InputField).GetText()
		host := form.GetFormItem(1).(*tview.InputField).GetText()
		port := form.GetFormItem(2).(*tview.InputField).GetText()
		login := form.GetFormItem(3).(*tview.InputField).GetText()
		pswd := form.GetFormItem(4).(*tview.InputField).GetText()

		if name == "" {
			slf.ta.OpenErrorModal(fmt.Errorf("Имя сервера должно быть заполнено"), slf.ShowServerListForm)
			return
		}

		if port == "" {
			slf.ta.OpenErrorModal(fmt.Errorf("Порт сервера должен быть заполнен"), slf.ShowServerListForm)
			return
		}

		if login == "" {
			slf.ta.OpenErrorModal(fmt.Errorf("Логин должен быть заполнен"), slf.ShowServerListForm)
			return
		}

		if pswd == "" {
			slf.ta.OpenErrorModal(fmt.Errorf("Пароль должен быть заполнен"), slf.ShowServerListForm)
			return
		}

		if register {
			check := form.GetFormItem(5).(*tview.InputField).GetText()

			if pswd != check {
				slf.ta.OpenErrorModal(fmt.Errorf("Пароли не совпадают"), slf.ShowServerListForm)
				return
			}
		}

		err := slf.lr.AddItem(name, host, port, login, pswd, register)
		if err != nil {
			slf.ta.OpenErrorModal(err, slf.ShowServerListForm)
			return
		}

		slf.ShowServerListForm()

	})

	form.AddButton("Отменить", func() {
		slf.ShowServerListForm()
	})

	form.SetBorder(true).SetTitle("Добавление сервера").SetTitleAlign(tview.AlignCenter)
	slf.ta.pages.AddPage(slf.name+"_new", form, true, true)
	slf.ta.pages.SwitchToPage(slf.name + "_new")

}

func (slf *ServerListForm) AskDeleteServerModal() {
	lri := slf.getSelectedRecord()
	if lri == nil {
		slf.ta.OpenErrorModal(fmt.Errorf("Нет выделенных записей"), slf.ShowServerListForm)
		return
	}

	modal := tview.NewModal().
		SetText(fmt.Sprintf("Удались данные с сервера (%s)?", lri.Name)).
		AddButtons([]string{"Да", "Нет"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			if buttonLabel == "Да" {
				slf.lr.DelItem(lri.Name)
			}
			slf.ShowServerListForm()
		})

	slf.ta.pages.AddPage(slf.name+"_delete", modal, true, true)
	slf.ta.pages.SwitchToPage(slf.name + "_delete")
}

func (slf *ServerListForm) ShowServerListForm() {
	slf.Draw()
	slf.ta.pages.SwitchToPage(slf.name)
	slf.ta.app.SetFocus(slf.table)
}

func (slf *ServerListForm) RedrawLoop(ctx context.Context, interval time.Duration) {

	logger.Log.Debug("ServerListForm.RedrawLoop START")
	defer logger.Log.Debug("ServerListForm.RedrawLoop STOP")

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Log.Debug("ServerListForm.RedrawLoop ctx.Done()")
			return
		case <-ticker.C:

			slf.ta.app.QueueUpdateDraw(func() {
				slf.Draw()
			})

		}
	}

}
