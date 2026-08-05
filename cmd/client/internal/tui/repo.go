package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/qesterrx/SafeKeeper/cmd/client/internal/model"
	"github.com/qesterrx/SafeKeeper/internal/logger"
	"github.com/rivo/tview"
)

type RepoService interface {
	GetItemList() ([]*model.IfaceObject, error)
	NewItem(name string, kind string) (*model.IfaceObject, error)
	EditItem(id int32) (*model.IfaceObject, []byte, error)
	CancelOperation(id int32) error
	UpdItem(id int32, data []byte) error
	DelItem(id int32) error
	GetTempFilename(id int32) (string, error)
	BackgroundProcess(ctx context.Context, interval time.Duration) error
}

type ObjectListForm struct {
	ta        *TUIApp
	table     *tview.Table
	statusBar *tview.TextView
	flex      *tview.Flex

	name string

	local RepoService

	ctx       context.Context
	ctxCancel context.CancelFunc
}

func ConstructObjectListForm(ta *TUIApp, local RepoService) (*ObjectListForm, error) {

	olf := ObjectListForm{}

	olf.ctx, olf.ctxCancel = context.WithCancel(ta.ctx)

	olf.ta = ta
	olf.local = local
	olf.name = "ObjectListForm"

	ta.wg.Go(func() {
		local.BackgroundProcess(olf.ctx, 10*time.Second)
	})

	ta.wg.Go(func() {
		olf.RedrawLoop(olf.ctx, 1*time.Second)
	})

	// Создание таблицы
	olf.table = tview.NewTable()
	olf.table.SetBorders(true)
	olf.table.SetSelectable(true, false) // Выбор только строк
	olf.table.SetFixed(1, 0)             // Фиксируем заголовок

	// Статусная строка с подсказками
	olf.statusBar = tview.NewTextView().
		SetTextColor(tcell.ColorYellow).
		SetText(" [+]Добавить  [-]Удалить  [*]Обновить      [Enter]Открыть  [Esc]Выход ")

	// Основной макет
	olf.flex = tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(olf.table, 0, 1, true).
		AddItem(olf.statusBar, 1, 0, false)

		// Обработка клавиш
	olf.flex.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case '+':
			olf.ShowNewObject()
			return nil
		case '-':
			olf.AskDeleteObject()
			return nil
		}

		switch event.Key() {
		case tcell.KeyEscape:
			olf.DestroyObjectListForm()
			return nil
		case tcell.KeyEnter:
			olf.ShowEditObject()
			return nil
		}
		return event
	})

	olf.ta.pages.AddPage(olf.name, olf.flex, true, true)
	return &olf, nil

}

func (olf *ObjectListForm) DestroyObjectListForm() {
	olf.ctxCancel()

	olf.ta.pages.RemovePage(olf.name)
	olf.ta.pages.SwitchToPage("ServerListForm")
}

func (olf *ObjectListForm) Draw() {
	olf.table.Clear()

	// Заголовки
	headers := []string{"ID", "Наименование", "Тип", "Версия изменения", "Дата обновления", "Статус", "Инфо"}
	for col, header := range headers {
		cell := tview.NewTableCell(header).
			SetTextColor(tcell.ColorYellow).
			SetAlign(tview.AlignCenter).
			SetSelectable(false)
		olf.table.SetCell(0, col, cell)
	}

	//Данные
	data, err := olf.local.GetItemList()
	if err != nil {
		olf.ta.OpenErrorModal(err, olf.ShowObjectListForm)
		return
	}

	for row, record := range data {

		col := 0
		olf.table.SetCell(row+1, 0, tview.NewTableCell(strconv.Itoa(int(record.Id))))
		olf.table.SetCell(row+1, col+1, tview.NewTableCell(record.Name))
		olf.table.SetCell(row+1, col+2, tview.NewTableCell(record.Kind))
		olf.table.SetCell(row+1, col+3, tview.NewTableCell(strconv.Itoa(int(record.Version))))
		olf.table.SetCell(row+1, col+4, tview.NewTableCell(record.Updated.Format("2006-01-02 15:04:05")))
		olf.table.SetCell(row+1, col+5, tview.NewTableCell(record.Status))
		olf.table.SetCell(row+1, col+6, tview.NewTableCell(record.Message))

		olf.table.GetCell(row+1, 0).SetReference(record)

	}
}

// --------------------------------------------------------------------------------
func (olf *ObjectListForm) getSelectedRecord() *model.IfaceObject {
	row, col := olf.table.GetSelection()

	if row >= 0 && col >= 0 {
		cell := olf.table.GetCell(row, col)
		if cell != nil {
			ref := cell.GetReference()
			if ref != nil {
				if obj, ok := ref.(*model.IfaceObject); ok {
					return obj
				}
			}
		}
	}
	return nil
}

// --------------------------------------------------------------------------------Создание нового объекта наименование и тип
func (olf *ObjectListForm) ShowNewObject() {

	form := tview.NewForm()

	form.AddInputField("Наименование: ", "", 50, nil, nil)
	form.AddDropDown("Тип объекта: ", model.GetListDataKind(), 50, nil)

	form.AddButton("Создать", func() {
		name := form.GetFormItem(0).(*tview.InputField).GetText()
		_, kind := form.GetFormItem(1).(*tview.DropDown).GetCurrentOption()

		if name == "" {
			olf.ta.OpenErrorModal(fmt.Errorf("Наименование объекта должно быть заполнено"), olf.ShowObjectListForm)
			return
		}

		if kind == "" {
			olf.ta.OpenErrorModal(fmt.Errorf("Наименование объекта должно быть заполнено"), olf.ShowObjectListForm)
			return
		}

		olf.ta.pages.RemovePage(olf.name + "_new")

		obj, err := olf.local.NewItem(name, kind)
		if err != nil {
			olf.ta.OpenErrorModal(err, olf.ShowObjectListForm)
			return
		}

		switch kind {
		case string(model.DataKindPswd):

			data := model.DOPwsd{Name: name}
			olf.ShowEditDataPasswd(obj, &data)
			return

		case string(model.DataKindCard):

			data := model.DOCard{Bank: name}
			olf.ShowEditDataCard(obj, &data)
			return

		case string(model.DataKindText):

			data := model.DOText{}
			olf.ShowEditDataText(obj, &data)
			return

		case string(model.DataKindBin):

			data := model.DOBin{}
			olf.ShowEditDataBin(obj, &data)
			return

		default:
			olf.ta.OpenErrorModal(fmt.Errorf("Данный клиент не умеет обрабатывать данный тип объекта"), olf.ShowObjectListForm)
			return
		}

	})

	form.AddButton("Отменить", func() {
		olf.ShowObjectListForm()
	})

	form.SetBorder(true).SetTitle("Добавление объекта").SetTitleAlign(tview.AlignCenter)
	olf.ta.pages.AddPage(olf.name+"_new", form, true, true)
	olf.ta.pages.SwitchToPage(olf.name + "_new")
}

// --------------------------------------------------------------------------------------------------ShowEditObject
func (olf *ObjectListForm) ShowEditObject() {

	rec := olf.getSelectedRecord()
	if rec == nil {
		olf.ta.OpenErrorModal(fmt.Errorf("Нет выделенных записей"), olf.ShowObjectListForm)
		return
	}

	obj, data, err := olf.local.EditItem(rec.Id)
	if err != nil {
		olf.ta.OpenErrorModal(err, olf.ShowObjectListForm)
		return
	}

	switch rec.Kind {
	case string(model.DataKindPswd):

		doPswd := model.DOPwsd{}
		err = json.Unmarshal(data, &doPswd)
		if err != nil {
			olf.ta.OpenErrorModal(fmt.Errorf("Ошибка чтения данных %w", err), olf.ShowObjectListForm)
			return
		}

		olf.ShowEditDataPasswd(obj, &doPswd)
		return
	case string(model.DataKindCard):

		doCard := model.DOCard{}
		err = json.Unmarshal(data, &doCard)
		if err != nil {
			olf.ta.OpenErrorModal(fmt.Errorf("Ошибка чтения данных %w", err), olf.ShowObjectListForm)
			return
		}

		olf.ShowEditDataCard(obj, &doCard)
		return
	case string(model.DataKindText):

		doText := model.DOText{}
		err = json.Unmarshal(data, &doText)
		if err != nil {
			olf.ta.OpenErrorModal(fmt.Errorf("Ошибка чтения данных %w", err), olf.ShowObjectListForm)
			return
		}

		olf.ShowEditDataText(obj, &doText)
		return

	case string(model.DataKindBin):

		doBin := model.DOBin{}
		err = json.Unmarshal(data, &doBin)
		if err != nil {
			olf.ta.OpenErrorModal(fmt.Errorf("Ошибка чтения данных %w", err), olf.ShowObjectListForm)
			return
		}

		olf.ShowEditDataBin(obj, &doBin)
		return

	default:
		olf.ta.OpenErrorModal(fmt.Errorf("Данный клиент не умеет обрабатывать данный тип объекта"), olf.ShowObjectListForm)
		return
	}

}

// --------------------------------------------------------------------------------ShowEditDataPasswd - Регистрация
func (olf *ObjectListForm) ShowEditDataPasswd(obj *model.IfaceObject, data *model.DOPwsd) {

	if data == nil {
		olf.ta.OpenErrorModal(fmt.Errorf("Ошибка открытия формы"), olf.ShowObjectListForm)
		return
	}

	form := tview.NewForm()

	form.AddInputField("Наименование: ", data.Name, 50, nil, nil)
	form.AddInputField("Логин: ", data.Login, 50, nil, nil)
	form.AddInputField("Пароль: ", data.Password, 50, nil, nil)
	form.AddTextArea("Комметарий: ", data.Comment, 50, 10, 0, nil)
	form.GetFormItem(3).(*tview.TextArea).SetOffset(0, 0)

	form.AddButton("Ок", func() {
		data.Name = form.GetFormItem(0).(*tview.InputField).GetText()
		data.Login = form.GetFormItem(1).(*tview.InputField).GetText()
		data.Password = form.GetFormItem(2).(*tview.InputField).GetText()
		data.Comment = form.GetFormItem(3).(*tview.TextArea).GetText()

		dataBytes, err := json.Marshal(data)
		if err != nil {
			olf.local.CancelOperation(obj.Id)
			olf.ta.OpenErrorModal(fmt.Errorf("Ошибка сохранения данных: %w", err), olf.ShowObjectListForm)
			return
		}

		err = olf.local.UpdItem(obj.Id, dataBytes)
		if err != nil {
			olf.local.CancelOperation(obj.Id)
			olf.ta.OpenErrorModal(err, olf.ShowObjectListForm)
			return
		}

		olf.ShowObjectListForm()

	})

	form.AddButton("Отменить", func() {
		olf.local.CancelOperation(obj.Id)
		olf.ShowObjectListForm()
	})

	form.SetBorder(true).SetTitle("Добавление сервера").SetTitleAlign(tview.AlignCenter)
	olf.ta.pages.AddPage(olf.name+"_password", form, true, true)
	olf.ta.pages.SwitchToPage(olf.name + "_password")

}

func (olf *ObjectListForm) ShowEditDataCard(obj *model.IfaceObject, data *model.DOCard) {

	if data == nil {
		olf.ta.OpenErrorModal(fmt.Errorf("Ошибка открытия формы"), olf.ShowObjectListForm)
		return
	}

	form := tview.NewForm()

	form.AddInputField("Банк: ", data.Bank, 50, nil, nil)
	form.AddInputField("ФИО: ", data.FIO, 50, nil, nil)
	form.AddInputField("Номер карты: ", data.Number, 50, nil, nil)
	form.AddInputField("ССV: ", data.CCV, 50, nil, nil)
	form.AddInputField("Срок действия: ", data.Date, 50, nil, nil)

	form.AddButton("Ок", func() {
		data.Bank = form.GetFormItem(0).(*tview.InputField).GetText()
		data.FIO = form.GetFormItem(1).(*tview.InputField).GetText()
		data.Number = form.GetFormItem(2).(*tview.InputField).GetText()
		data.CCV = form.GetFormItem(3).(*tview.InputField).GetText()
		data.Date = form.GetFormItem(4).(*tview.InputField).GetText()

		dataBytes, err := json.Marshal(data)
		if err != nil {
			olf.local.CancelOperation(obj.Id)
			olf.ta.OpenErrorModal(fmt.Errorf("Ошибка сохранения данных: %w", err), olf.ShowObjectListForm)
			return
		}

		err = olf.local.UpdItem(obj.Id, dataBytes)
		if err != nil {
			olf.local.CancelOperation(obj.Id)
			olf.ta.OpenErrorModal(err, olf.ShowObjectListForm)
			return
		}

		olf.ShowObjectListForm()

	})

	form.AddButton("Отменить", func() {
		olf.local.CancelOperation(obj.Id)
		olf.ShowObjectListForm()
	})

	form.SetBorder(true).SetTitle("Добавление сервера").SetTitleAlign(tview.AlignCenter)
	olf.ta.pages.AddPage(olf.name+"_card", form, true, true)
	olf.ta.pages.SwitchToPage(olf.name + "_card")

}

func (olf *ObjectListForm) ShowEditDataText(obj *model.IfaceObject, data *model.DOText) {

	if data == nil {
		olf.ta.OpenErrorModal(fmt.Errorf("Ошибка открытия формы"), olf.ShowObjectListForm)
		return
	}

	form := tview.NewForm()

	form.AddTextArea("Текст: ", data.Text, 50, 10, 0, nil)
	form.GetFormItem(0).(*tview.TextArea).SetOffset(0, 0)

	form.AddButton("Ок", func() {
		data.Text = form.GetFormItem(0).(*tview.TextArea).GetText()

		dataBytes, err := json.Marshal(data)
		if err != nil {
			olf.local.CancelOperation(obj.Id)
			olf.ta.OpenErrorModal(fmt.Errorf("Ошибка сохранения данных: %w", err), olf.ShowObjectListForm)
			return
		}

		err = olf.local.UpdItem(obj.Id, dataBytes)
		if err != nil {
			olf.local.CancelOperation(obj.Id)
			olf.ta.OpenErrorModal(err, olf.ShowObjectListForm)
			return
		}

		olf.ShowObjectListForm()

	})

	form.AddButton("Отменить", func() {
		olf.local.CancelOperation(obj.Id)
		olf.ShowObjectListForm()
	})

	form.SetBorder(true).SetTitle("Добавление сервера").SetTitleAlign(tview.AlignCenter)
	olf.ta.pages.AddPage(olf.name+"_text", form, true, true)
	olf.ta.pages.SwitchToPage(olf.name + "_text")

}

func (olf *ObjectListForm) ShowEditDataBin(obj *model.IfaceObject, data *model.DOBin) {

	if data == nil {
		olf.ta.OpenErrorModal(fmt.Errorf("Ошибка открытия формы"), olf.ShowObjectListForm)
		return
	}

	if len(data.Data) == 0 {
		//Выбрать файл

		form := tview.NewForm()

		form.AddInputField("Файл: ", "", 50, nil, nil)
		form.AddButton("Ок", func() {

			filename := form.GetFormItem(0).(*tview.InputField).GetText()

			dataFile, err := os.ReadFile(filename)
			if err != nil {
				olf.local.CancelOperation(obj.Id)
				olf.ta.OpenErrorModal(fmt.Errorf("Ошибка сохранения данных: %w", err), olf.ShowObjectListForm)
				return
			}

			data.Data = dataFile
			data.Ext = filepath.Ext(filename)

			dataBytes, err := json.Marshal(data)
			if err != nil {
				olf.local.CancelOperation(obj.Id)
				olf.ta.OpenErrorModal(fmt.Errorf("Ошибка сохранения данных: %w", err), olf.ShowObjectListForm)
				return
			}

			err = olf.local.UpdItem(obj.Id, dataBytes)
			if err != nil {
				olf.local.CancelOperation(obj.Id)
				olf.ta.OpenErrorModal(err, olf.ShowObjectListForm)
				return
			}

			olf.ShowObjectListForm()
		})

		form.AddButton("Отменить", func() {
			olf.local.CancelOperation(obj.Id)
			olf.ShowObjectListForm()
		})

		form.SetBorder(true).SetTitle("Добавление сервера").SetTitleAlign(tview.AlignCenter)
		olf.ta.pages.AddPage(olf.name+"_text", form, true, true)
		olf.ta.pages.SwitchToPage(olf.name + "_text")

	} else {
		//Открыть файл
		filename, err := olf.local.GetTempFilename(obj.Id)
		if err != nil {
			olf.local.CancelOperation(obj.Id)
			olf.ta.OpenErrorModal(err, olf.ShowObjectListForm)
			return
		}

		fullFilename := filename + data.Ext
		err = os.WriteFile(fullFilename, data.Data, 0644)
		if err != nil {
			olf.local.CancelOperation(obj.Id)
			olf.ta.OpenErrorModal(err, olf.ShowObjectListForm)
			return
		}

		OpenFile := func(filename string) error {
			var cmd *exec.Cmd

			switch runtime.GOOS {
			case "windows":
				cmd = exec.Command("cmd", "/c", "start", "/wait", filename)
			case "darwin": // macOS
				cmd = exec.Command("open", "-W", filename)
			case "linux":
				cmd = exec.Command("xdg-open", filename)
			default:
				return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
			}

			// Запускаем и ждем завершения
			return cmd.Run()
		}

		err = OpenFile(fullFilename)
		if err != nil {
			os.Remove(fullFilename)
			olf.local.CancelOperation(obj.Id)
			olf.ta.OpenErrorModal(err, olf.ShowObjectListForm)
			return
		}

		time.Sleep(1 * time.Second)

		olf.local.CancelOperation(obj.Id)
		os.Remove(fullFilename)
		olf.ShowObjectListForm()

	}

}

func (olf *ObjectListForm) AskDeleteObject() {
	rec := olf.getSelectedRecord()
	if rec == nil {
		olf.ta.OpenErrorModal(fmt.Errorf("Нет выделенных записей"), olf.ShowObjectListForm)
		return
	}

	modal := tview.NewModal().
		SetText(fmt.Sprintf("Удались объект (%d: %s)?", rec.Id, rec.Name)).
		AddButtons([]string{"Да", "Нет"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			if buttonLabel == "Да" {
				err := olf.local.DelItem(rec.Id)
				if err != nil {
					olf.ta.OpenErrorModal(err, olf.ShowObjectListForm)
					return
				}
			}
			olf.ShowObjectListForm()
		})

	olf.ta.pages.AddPage(olf.name+"_delete", modal, true, true)
	olf.ta.pages.SwitchToPage(olf.name + "_delete")
}

func (olf *ObjectListForm) ShowObjectListForm() {
	olf.Draw()
	olf.ta.pages.SwitchToPage(olf.name)
	olf.ta.app.SetFocus(olf.table)
}

func (olf *ObjectListForm) RedrawLoop(ctx context.Context, interval time.Duration) {

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Log.Debug("SignalWorkerLoop ctx.Done()")
			return
		case <-ticker.C:

			olf.ta.app.QueueUpdateDraw(func() {
				olf.Draw()
			})

		}
	}

}
