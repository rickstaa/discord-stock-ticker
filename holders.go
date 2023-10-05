package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/rssnyder/discord-stock-ticker/utils"
)

// Holders represents the json for holders
type Holders struct {
	Network   string   `json:"network"`
	Address   string   `json:"address"`
	Activity  string   `json:"activity"`
	Nickname  bool     `json:"nickname"`
	Frequency int      `json:"frequency"`
	APIToken  string   `json:"api_token"`
	ClientID  string   `json:"client_id"`
	Token     string   `json:"discord_bot_token"`
	close     chan int `json:"-"`
}

// label returns a human readble id for this bot
func (h *Holders) label() string {
	label := strings.ToLower(fmt.Sprintf("%s-%s", h.Network, h.Address))
	if len(label) > 32 {
		label = label[:32]
	}
	return label
}

// Shutdown sends a signal to shut off the goroutine
func (h *Holders) Shutdown() {
	h.close <- 1
}

func (h *Holders) watchHolders() {

	// create a new discord session using the provided bot token.
	dg, err := discordgo.New("Bot " + h.Token)
	if err != nil {
		logger.Errorf("Creating Discord session (%s): %s", h.ClientID, err)
		lastUpdate.With(prometheus.Labels{"type": "holders", "ticker": fmt.Sprintf("%s-%s", h.Network, h.Address), "guild": "None"}).Set(0)
		return
	}

	// show as online
	err = dg.Open()
	if err != nil {
		logger.Errorf("Opening discord connection (%s): %s", h.ClientID, err)
		lastUpdate.With(prometheus.Labels{"type": "holders", "ticker": fmt.Sprintf("%s-%s", h.Network, h.Address), "guild": "None"}).Set(0)
		return
	}

	// set activity as desc
	if h.Nickname {
		err = dg.UpdateWatchStatus(0, h.Activity)
		if err != nil {
			logger.Errorf("Unable to set activity: %s\n", err)
		} else {
			logger.Debugf("Set activity")
		}
	}

	// get guides for bot
	guilds, err := dg.UserGuilds(100, "", "")
	if err != nil {
		logger.Errorf("Error getting guilds: %s\n", err)
		h.Nickname = false
	}
	if len(guilds) == 0 {
		h.Nickname = false
	}

	// check for frequency override
	// set to one hour to avoid lockout
	if *frequency != 0 {
		h.Frequency = *frequency
	}

	// perform management operations
	if *managed {
		setName(dg, h.label())
	}

	logger.Infof("Watching holders for %s", h.Address)
	ticker := time.NewTicker(time.Duration(h.Frequency) * time.Second)

	h.close = make(chan int, 1)

	for {

		select {
		case <-h.close:
			logger.Infof("Shutting down price watching for %s", h.Activity)
			return
		case <-ticker.C:

			holders, err := utils.GetHolders(h.Network, h.Address, h.APIToken)
			if err != nil {
				logger.Errorf("Error getting holders for %s/%s %s", h.Network, h.Address, err)
				continue
			}
			displayName := fmt.Sprintf("%d", holders)

			if h.Nickname {

				for _, g := range guilds {

					err = dg.GuildMemberNickname(g.ID, "@me", displayName)
					if err != nil {
						logger.Errorf("Error updating nickname: %s\n", err)
						continue
					} else {
						logger.Debugf("Set nickname in %s: %s\n", g.Name, displayName)
					}
					lastUpdate.With(prometheus.Labels{"type": "holders", "ticker": fmt.Sprintf("%s-%s", h.Network, h.Address), "guild": g.Name}).SetToCurrentTime()
					time.Sleep(time.Duration(h.Frequency) * time.Second)
				}
			} else {

				err = dg.UpdateWatchStatus(0, displayName)
				if err != nil {
					logger.Errorf("Unable to set activity: %s\n", err)
				} else {
					logger.Debugf("Set activity: %s\n", displayName)
					lastUpdate.With(prometheus.Labels{"type": "holders", "ticker": fmt.Sprintf("%s-%s", h.Network, h.Address), "guild": "None"}).SetToCurrentTime()
				}
			}
		}
	}
}
