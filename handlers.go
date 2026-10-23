package main

import (
	"errors"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/AshokShau/gotdbot"
)

const helpText = "<b>MongoDB Backup Bot</b>\n\n" +
	"<b>Back up</b>\n<code>/mongo &lt;URI&gt;</code> — compressed archive\n" +
	"<code>/mongo &lt;URI&gt; {zip}</code> — ZIP with BSON and metadata\n" +
	"<i>{json} is an alias for {zip}, not a plain JSON export.</i>\n\n" +
	"<b>Restore</b>\nReply to a <code>.gz</code> or <code>.zip</code> backup with " +
	"<code>/mongo &lt;destination_URI&gt; {import}</code>, then confirm.\n\n" +
	"Choose a database or back up all from the menu. Wipe requires confirmation.\n" +
	"Use database commands in a private chat. /ping shows uptime."

const privacyText = "<b>Data handling</b>\n\n" +
	"The bot reads your URI from your command when you press a button. " +
	"Commands and backup documents remain in Telegram until you delete them.\n\n" +
	"Backups and downloads use temporary files on the server, removed after processing. " +
	"TDLib keeps its session on disk. A crash can leave temporary files.\n\n" +
	"URIs are omitted from backup captions and tool logs. Use a trusted deployment."

func startCommand(c *gotdbot.Client, m *gotdbot.Message) error {
	_, err := m.ReplyText(c, helpText, &gotdbot.SendMessageOpts{ReplyMarkup: homeKeyboard("help"), DisableWebPagePreview: true})
	return err
}

func privacyCommand(c *gotdbot.Client, m *gotdbot.Message) error {
	_, err := m.ReplyText(c, privacyText, &gotdbot.SendMessageOpts{ReplyMarkup: homeKeyboard("privacy")})
	return err
}

func pingCommand(c *gotdbot.Client, m *gotdbot.Message) error {
	start := time.Now()
	reply, err := m.ReplyText(c, "Pinging…", nil)
	if err != nil {
		return err
	}
	_, err = reply.EditText(c, fmt.Sprintf("<b>Pong</b>\n\n<b>Latency:</b> <code>%d ms</code>\n<b>Uptime:</b> <code>%s</code>", time.Since(start).Milliseconds(), time.Since(started).Round(time.Second)), nil)
	return err
}

func helpCallback(c *gotdbot.Client, q *gotdbot.UpdateNewCallbackQuery) error {
	if err := q.Answer(c, "", nil); err != nil {
		return err
	}
	_, err := q.EditMessageText(c, helpText, &gotdbot.EditMessageTextOpts{ReplyMarkup: homeKeyboard("help")})
	return err
}

func privacyCallback(c *gotdbot.Client, q *gotdbot.UpdateNewCallbackQuery) error {
	if err := q.Answer(c, "", nil); err != nil {
		return err
	}
	_, err := q.EditMessageText(c, privacyText, &gotdbot.EditMessageTextOpts{ReplyMarkup: homeKeyboard("privacy")})
	return err
}

func mongoCommand(c *gotdbot.Client, m *gotdbot.Message) error {
	_, format, importing, err := parseMongoRequest(m.Text())
	if err != nil {
		_, err = m.ReplyText(c, html.EscapeString(err.Error()), nil)
		return err
	}
	text, keyboard := "<b>MongoDB</b>\n\nChoose an action.\n<b>Format:</b> <code>"+strings.ToUpper(format)+"</code>", menuKeyboard()
	if importing {
		document, err := m.GetRepliedMessage(c)
		if err != nil || !backupDocument(document) {
			_, err = m.ReplyText(c, "Reply to a <code>.gz</code> archive or <code>.zip</code> BSON dump.", nil)
			return err
		}
		text, keyboard = "<b>Restore this backup?</b>\n\nDatabase names come from the backup. Existing collections are kept; errors can leave a partial restore.", confirmKeyboard("restore")
	}
	_, err = m.ReplyText(c, text, &gotdbot.SendMessageOpts{ReplyMarkup: keyboard})
	return err
}

func backupDocument(m *gotdbot.Message) bool {
	if m == nil {
		return false
	}
	doc, ok := m.Content.(*gotdbot.MessageDocument)
	if !ok || doc.Document == nil {
		return false
	}
	ext := strings.ToLower(filepath.Ext(doc.Document.FileName))
	return ext == ".gz" || ext == ".zip"
}

func mongoCallback(c *gotdbot.Client, q *gotdbot.UpdateNewCallbackQuery) error {
	menu, err := q.GetMessage(c)
	if err != nil || menu == nil {
		return q.Answer(c, "Menu unavailable. Send /mongo again.", nil)
	}
	command, err := menu.GetRepliedMessage(c)
	if err != nil || command == nil || command.SenderID() != q.SenderUserId {
		return q.Answer(c, "This menu belongs to another user or its command was deleted.", &gotdbot.CallbackAnswerOpts{ShowAlert: true})
	}

	var selected string
	if keyboard, ok := menu.ReplyMarkup.(*gotdbot.ReplyMarkupInlineKeyboard); ok {
		for _, row := range keyboard.Rows {
			for _, key := range row {
				if data, ok := key.Type.(*gotdbot.InlineKeyboardButtonTypeCallback); ok && string(data.Data) == q.DataString() {
					selected = key.Text
				}
			}
		}
	}

	if selected == "" {
		return q.Answer(c, "This button is no longer active.", nil)
	}
	if err := q.Answer(c, "", nil); err != nil {
		return err
	}
	uri, format, _, err := parseMongoRequest(command.Text())
	if err != nil {
		return err
	}
	data := strings.TrimPrefix(q.DataString(), "mongo:")
	switch {
	case data == "cancel":
		_, err = q.EditMessageText(c, "<b>Operation cancelled.</b>", nil)
	case data == "menu":
		_, err = q.EditMessageText(c, "<b>Choose an action</b>", &gotdbot.EditMessageTextOpts{ReplyMarkup: menuKeyboard()})
	case strings.HasPrefix(data, "page:"):
		page, parseErr := strconv.Atoi(strings.TrimPrefix(data, "page:"))
		if parseErr != nil {
			return parseErr
		}
		names, listErr := databaseNames(uri)
		if listErr != nil {
			err = listErr
		} else if len(names) == 0 {
			err = errors.New("no user databases found")
		} else {
			_, err = q.EditMessageText(c, "<b>Select a database</b>", &gotdbot.EditMessageTextOpts{ReplyMarkup: databaseKeyboard(names, page)})
		}
	case data == "wipe":
		_, err = q.EditMessageText(c, "<b>Wipe all user databases in this URI's scope?</b>\n\nThis permanently deletes their data. System databases are excluded. This cannot be undone.", &gotdbot.EditMessageTextOpts{ReplyMarkup: confirmKeyboard("wipe")})
	case data == "wipe:confirm":
		if _, err = q.EditMessageText(c, "<b>Wiping user databases…</b>", nil); err == nil {
			err = wipeDatabases(uri)
			if err == nil {
				_, err = q.EditMessageText(c, "<b>User databases wiped.</b>", nil)
			}
		}
	case data == "restore:confirm":
		err = importBackup(c, q, command)
	case data == "all" || strings.HasPrefix(data, "db:"):
		database := ""
		if data != "all" {
			database = selected
		}
		err = sendBackup(c, q, uri, format, database)
	}
	if err != nil {
		_, editErr := q.EditMessageText(c, "<b>Operation failed.</b>\n\n"+html.EscapeString(err.Error())+"\n\nA restore or wipe may have applied partially.", nil)
		return editErr
	}
	return nil
}

func sendBackup(c *gotdbot.Client, q *gotdbot.UpdateNewCallbackQuery, uri, format, database string) error {
	if _, err := q.EditMessageText(c, "<b>Creating backup…</b>", nil); err != nil {
		return err
	}
	dir, err := os.MkdirTemp("backups", "backup-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	path, err := dumpMongo(uri, format, database, dir)
	if err != nil {
		return err
	}
	label := database
	if label == "" {
		label = "All databases in URI scope"
	}

	_, err = c.SendDocument(q.ChatId, &gotdbot.InputFileLocal{Path: path}, &gotdbot.SendDocumentOpts{
		Caption:          "<b>MongoDB backup</b>\n\n<b>Database:</b> <code>" + html.EscapeString(label) + "</code>\n<b>Format:</b> <code>" + strings.ToUpper(format) + "</code>",
		ReplyToMessageID: q.MessageId, DisableContentTypeDetection: true,
	})

	if err != nil {
		return err
	}

	_, err = q.EditMessageText(c, "<b>Backup sent.</b>", nil)
	return err
}

func importBackup(c *gotdbot.Client, q *gotdbot.UpdateNewCallbackQuery, command *gotdbot.Message) error {
	document, err := command.GetRepliedMessage(c)
	if err != nil || !backupDocument(document) {
		return errors.New("backup document is unavailable")
	}
	if _, err := q.EditMessageText(c, "<b>Downloading and restoring backup…</b>", nil); err != nil {
		return err
	}
	file, err := document.Download(c, 16, &gotdbot.DownloadFileOpts{Synchronous: true})
	if err != nil {
		return err
	}
	defer c.DeleteFile(file.Id)
	if file.Local == nil || !file.Local.IsDownloadingCompleted {
		return errors.New("backup download did not complete")
	}
	defer os.Remove(file.Local.Path)
	dir, err := os.MkdirTemp("backups", "restore-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	uri, _, _, err := parseMongoRequest(command.Text())
	if err != nil {
		return err
	}
	ext := strings.ToLower(filepath.Ext(document.Content.(*gotdbot.MessageDocument).Document.FileName))
	if err := restoreMongo(uri, file.Local.Path, ext, dir); err != nil {
		return err
	}
	_, err = q.EditMessageText(c, "<b>Restore complete.</b>", nil)
	return err
}
