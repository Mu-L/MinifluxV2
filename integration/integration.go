// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package integration // import "miniflux.app/v2/integration"

import (
	"miniflux.app/v2/config"
	"miniflux.app/v2/integration/apprise"
	"miniflux.app/v2/integration/espial"
	"miniflux.app/v2/integration/instapaper"
	"miniflux.app/v2/integration/linkding"
	"miniflux.app/v2/integration/matrixbot"
	"miniflux.app/v2/integration/notion"
	"miniflux.app/v2/integration/nunuxkeeper"
	"miniflux.app/v2/integration/pinboard"
	"miniflux.app/v2/integration/pocket"
	"miniflux.app/v2/integration/readwise"
	"miniflux.app/v2/integration/telegrambot"
	"miniflux.app/v2/integration/wallabag"
	"miniflux.app/v2/logger"
	"miniflux.app/v2/model"
)

// SendEntry sends the entry to third-party providers when the user click on "Save".
func SendEntry(entry *model.Entry, integration *model.Integration) {
	if integration.PinboardEnabled {
		logger.Debug("[Integration] Sending Entry #%d %q for User #%d to Pinboard", entry.ID, entry.URL, integration.UserID)

		client := pinboard.NewClient(integration.PinboardToken)
		err := client.AddBookmark(
			entry.URL,
			entry.Title,
			integration.PinboardTags,
			integration.PinboardMarkAsUnread,
		)

		if err != nil {
			logger.Error("[Integration] UserID #%d: %v", integration.UserID, err)
		}
	}

	if integration.InstapaperEnabled {
		logger.Debug("[Integration] Sending Entry #%d %q for User #%d to Instapaper", entry.ID, entry.URL, integration.UserID)

		client := instapaper.NewClient(integration.InstapaperUsername, integration.InstapaperPassword)
		if err := client.AddURL(entry.URL, entry.Title); err != nil {
			logger.Error("[Integration] UserID #%d: %v", integration.UserID, err)
		}
	}

	if integration.WallabagEnabled {
		logger.Debug("[Integration] Sending Entry #%d %q for User #%d to Wallabag", entry.ID, entry.URL, integration.UserID)

		client := wallabag.NewClient(
			integration.WallabagURL,
			integration.WallabagClientID,
			integration.WallabagClientSecret,
			integration.WallabagUsername,
			integration.WallabagPassword,
			integration.WallabagOnlyURL,
		)

		if err := client.AddEntry(entry.URL, entry.Title, entry.Content); err != nil {
			logger.Error("[Integration] UserID #%d: %v", integration.UserID, err)
		}
	}

	if integration.NotionEnabled {
		logger.Debug("[Integration] Sending Entry #%d %q for User #%d to Notion", entry.ID, entry.URL, integration.UserID)

		client := notion.NewClient(
			integration.NotionToken,
			integration.NotionPageID,
		)
		if err := client.AddEntry(entry.URL, entry.Title); err != nil {
			logger.Error("[Integration] UserID #%d: %v", integration.UserID, err)
		}
	}

	if integration.NunuxKeeperEnabled {
		logger.Debug("[Integration] Sending Entry #%d %q for User #%d to NunuxKeeper", entry.ID, entry.URL, integration.UserID)

		client := nunuxkeeper.NewClient(
			integration.NunuxKeeperURL,
			integration.NunuxKeeperAPIKey,
		)

		if err := client.AddEntry(entry.URL, entry.Title, entry.Content); err != nil {
			logger.Error("[Integration] UserID #%d: %v", integration.UserID, err)
		}
	}

	if integration.EspialEnabled {
		logger.Debug("[Integration] Sending Entry #%d %q for User #%d to Espial", entry.ID, entry.URL, integration.UserID)

		client := espial.NewClient(
			integration.EspialURL,
			integration.EspialAPIKey,
		)

		if err := client.AddEntry(entry.URL, entry.Title, entry.Content, integration.EspialTags); err != nil {
			logger.Error("[Integration] UserID #%d: %v", integration.UserID, err)
		}
	}

	if integration.PocketEnabled {
		logger.Debug("[Integration] Sending Entry #%d %q for User #%d to Pocket", entry.ID, entry.URL, integration.UserID)

		client := pocket.NewClient(config.Opts.PocketConsumerKey(integration.PocketConsumerKey), integration.PocketAccessToken)
		if err := client.AddURL(entry.URL, entry.Title); err != nil {
			logger.Error("[Integration] UserID #%d: %v", integration.UserID, err)
		}
	}

	if integration.LinkdingEnabled {
		logger.Debug("[Integration] Sending Entry #%d %q for User #%d to Linkding", entry.ID, entry.URL, integration.UserID)

		client := linkding.NewClient(
			integration.LinkdingURL,
			integration.LinkdingAPIKey,
			integration.LinkdingTags,
			integration.LinkdingMarkAsUnread,
		)
		if err := client.AddEntry(entry.Title, entry.URL); err != nil {
			logger.Error("[Integration] UserID #%d: %v", integration.UserID, err)
		}
	}

	if integration.ReadwiseEnabled {
		logger.Debug("[Integration] Sending Entry #%d %q for User #%d to Readwise Reader", entry.ID, entry.URL, integration.UserID)

		client := readwise.NewClient(
			integration.ReadwiseAPIKey,
		)

		if err := client.AddEntry(entry.URL); err != nil {
			logger.Error("[Integration] UserID #%d: %v", integration.UserID, err)
		}
	}
}

// PushEntries pushes an entry array to third-party providers during feed refreshes.
func PushEntries(entries model.Entries, integration *model.Integration) {
	if integration.MatrixBotEnabled {
		logger.Debug("[Integration] Sending %d entries for User #%d to Matrix", len(entries), integration.UserID)

		err := matrixbot.PushEntries(entries, integration.MatrixBotURL, integration.MatrixBotUser, integration.MatrixBotPassword, integration.MatrixBotChatID)
		if err != nil {
			logger.Error("[Integration] push entries to matrix bot failed: %v", err)
		}
	}
}

// PushEntry pushes an entry to third-party providers during feed refreshes.
func PushEntry(entry *model.Entry, integration *model.Integration) {
	if integration.TelegramBotEnabled {
		logger.Debug("[Integration] Sending Entry %q for User #%d to Telegram", entry.URL, integration.UserID)

		err := telegrambot.PushEntry(entry, integration.TelegramBotToken, integration.TelegramBotChatID)
		if err != nil {
			logger.Error("[Integration] push entry to telegram bot failed: %v", err)
		}
	}
	if integration.AppriseEnabled {
		logger.Debug("[Integration] Sending Entry %q for User #%d to apprise", entry.URL, integration.UserID)

		client := apprise.NewClient(
			integration.AppriseServicesURL,
			integration.AppriseURL,
		)
		err := client.PushEntry(entry)
		if err != nil {
			logger.Error("[Integration] push entry to apprise failed: %v", err)
		}
	}
}
