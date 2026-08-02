package tui

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/qesterrx/SafeKeeper/cmd/client/internal/config"
	"github.com/qesterrx/SafeKeeper/cmd/client/internal/model"
	"github.com/qesterrx/SafeKeeper/cmd/client/internal/service"
	"github.com/qesterrx/SafeKeeper/cmd/client/internal/transport"
	"github.com/qesterrx/SafeKeeper/internal/logger"

	"github.com/rivo/tview"
)

type TUIApp struct {
	app   *tview.Application
	pages *tview.Pages

	cnfig *config.Configuration

	wg        sync.WaitGroup
	ctx       context.Context
	ctxCancel context.CancelFunc
}

func NewTUIApp(ctx context.Context, cnfig *config.Configuration) (*TUIApp, error) {

	ta := TUIApp{}

	ta.ctx, ta.ctxCancel = context.WithCancel(ctx)

	ta.app = tview.NewApplication()
	ta.pages = tview.NewPages()
	ta.cnfig = cnfig

	//Создаем объекты сервисного слоя
	server := transport.GRPCServer{}
	lr, err := service.NewListRepository(ta.ctx, &server, cnfig)
	if err != nil {
		return nil, err
	}
	//Создаем форму
	slf, err := ConstructServerListForm(&ta, lr)
	if err != nil {
		return nil, err
	}

	//Показываем форму
	slf.ShowServerListForm()

	return &ta, nil

}

func (ta *TUIApp) StartTUIApp() error {
	return ta.app.SetRoot(ta.pages, true).Run()
}

func (ta *TUIApp) StopTUIApp() {
	logger.Log.Debug("StopTUIApp start")
	ta.ctxCancel()

	//Перестраховываемся
	done := make(chan struct{})
	go func() {
		ta.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		logger.Log.Debug("Wait fin gorutine - success")
	case <-time.After(2 * time.Second):
		logger.Log.Debug("Wait fin gorutine - too long")
	}

	ta.app.Stop()
	logger.Log.Debug("StopTUIAppFinish")
}

func (ta *TUIApp) OpenErrorModal(err error, receiver func()) {

	if err == nil {
		return
	}

	modal := tview.NewModal().
		SetText(fmt.Sprintf("[red]Ошибка[::-]\n\n%s", err.Error())).
		AddButtons([]string{"OK"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			// Возвращаемся к основному представлению
			ta.pages.RemovePage("ErrorModal")
			if receiver != nil {
				receiver()
			}
		})

	// Показываем модальное окно поверх всего
	ta.pages.AddPage("ErrorModal", modal, true, true)
	ta.pages.SwitchToPage("ErrorModal")
}

func (ta *TUIApp) OpenPasswordModal(lri *model.IfaceRepoItem, receiver func()) {
	form := tview.NewForm()

	passwordInput := tview.NewInputField().
		SetLabel("Пароль: ").
		SetFieldWidth(30).
		SetMaskCharacter('*').
		SetAcceptanceFunc(tview.InputFieldMaxLength(30))
	form.AddFormItem(passwordInput)

	form.AddButton("Открыть", func() {
		pswd := form.GetFormItem(0).(*tview.InputField).GetText()

		local, err := service.NewLocalReposityory(lri, pswd, ta.cnfig)

		if err != nil {
			ta.OpenErrorModal(err, receiver)
			return
		}

		olf, err := ConstructObjectListForm(ta, local)
		if err != nil {
			ta.OpenErrorModal(err, receiver)
			return
		}

		ta.pages.RemovePage("password")
		olf.ShowObjectListForm()

	})

	form.AddButton("Отменить", func() {
		receiver()
	})

	form.SetBorder(true).SetTitle("Авторизация").SetTitleAlign(tview.AlignCenter)
	ta.pages.AddPage("password", form, true, true)
	ta.pages.SwitchToPage("password")

}
