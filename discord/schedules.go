package discord

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bwmarrin/discordgo"
	"github.com/robfig/cron/v3"
)

// BotScheduleI defines the interface for scheduled tasks in the bot
type BotScheduleI interface {
	// GetName returns the name of the schedule
	GetName() string
	// GetCronExpression returns the cron expression for when this schedule should run
	GetCronExpression() string
	// Execute runs the scheduled task and returns an embed to send (or nil if no notification needed)
	Execute() (*discordgo.MessageEmbed, error)
}

// GenericBotSchedule is a generic implementation of BotScheduleI
type GenericBotSchedule struct {
	// Name is the schedule's identifier
	Name string
	// CronExpression determines when the schedule will execute
	CronExpression string
	// Handler is the function to execute on schedule
	Handler func() (*discordgo.MessageEmbed, error)
}

// GetName returns the schedule's name
func (bs *GenericBotSchedule) GetName() string {
	return bs.Name
}

// GetCronExpression returns the schedule's cron expression
func (bs *GenericBotSchedule) GetCronExpression() string {
	return bs.CronExpression
}

// Execute runs the scheduled task
func (bs *GenericBotSchedule) Execute() (*discordgo.MessageEmbed, error) {
	return bs.Handler()
}

// NewBotSchedule creates a new scheduled task with the given name, cron expression, and handler
func NewBotSchedule(name string, cronExpr string, handler func() (*discordgo.MessageEmbed, error)) BotScheduleI {
	return &GenericBotSchedule{
		Name:           name,
		CronExpression: cronExpr,
		Handler:        handler,
	}
}

// scheduleManager handles scheduling and executing tasks
type scheduleManager struct {
	bot        *Bot
	cron       *cron.Cron
	schedules  []BotScheduleI
	ctx        context.Context
	cancelFunc context.CancelFunc
}

// newScheduleManager creates a new scheduleManager
func newScheduleManager(bot *Bot, schedules []BotScheduleI) *scheduleManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &scheduleManager{
		bot:        bot,
		cron:       cron.New(cron.WithSeconds()),
		schedules:  schedules,
		ctx:        ctx,
		cancelFunc: cancel,
	}
}

// start initializes and starts all scheduled tasks
func (sm *scheduleManager) start() error {
	for _, schedule := range sm.schedules {
		// Use closure to capture the schedule
		sched := schedule
		_, err := sm.cron.AddFunc(sched.GetCronExpression(), func() {
			sm.executeSchedule(sched)
		})
		if err != nil {
			return fmt.Errorf("failed to add schedule %s: %w", sched.GetName(), err)
		}
		slog.Info("registered schedule", "name", sched.GetName(), "cron", sched.GetCronExpression())
	}

	sm.cron.Start()
	slog.Info("schedule manager started", "schedules", len(sm.schedules))
	return nil
}

// executeSchedule runs a scheduled task and sends notifications if needed
func (sm *scheduleManager) executeSchedule(schedule BotScheduleI) {
	slog.Debug("executing schedule", "name", schedule.GetName(), "cron", schedule.GetCronExpression())

	embed, err := schedule.Execute()
	if err != nil {
		slog.Error("failed to execute schedule",
			"name", schedule.GetName(),
			"error", err)
		return
	}

	// If the embed is nil, no notification is needed
	if embed == nil {
		return
	}

	// Send the embed to all guilds
	for _, guild := range sm.bot.session.State.Guilds {
		// Find the first text channel to send the notification
		channels, err := sm.bot.session.GuildChannels(guild.ID)
		if err != nil {
			slog.Error("failed to get guild channels",
				"guild", guild.ID,
				"error", err)
			continue
		}

		var targetChannel string
		for _, channel := range channels {
			if channel.Type == discordgo.ChannelTypeGuildText {
				targetChannel = channel.ID
				break
			}
		}

		if targetChannel != "" {
			_, err = sm.bot.session.ChannelMessageSendEmbed(targetChannel, embed)
			if err != nil {
				slog.Error("failed to send schedule notification",
					"guild", guild.ID,
					"schedule", schedule.GetName(),
					"error", err)
			}
		}
	}
}

// stop cleanly shuts down the scheduler
func (sm *scheduleManager) stop() {
	sm.cancelFunc()
	sm.cron.Stop()
	slog.Info("schedule manager stopped")
}
