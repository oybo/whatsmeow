// Copyright (c) 2026 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"math/rand"
	"time"

	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// AutoReceiptConfig 控制收到消息后的自动已读和 presence 订阅行为。
// 普通 delivered 回执仍然在消息成功解密后的核心协议路径里处理。
type AutoReceiptConfig struct {
	Enabled bool

	SendRead          bool
	SubscribePresence bool
	IncludeGroups     bool

	ReadDelayMin time.Duration
	ReadDelayMax time.Duration
}

// DefaultAutoReceiptConfig 返回一组可以直接启用的自动回执配置。
func DefaultAutoReceiptConfig() AutoReceiptConfig {
	return AutoReceiptConfig{
		Enabled:           true,
		SendRead:          true,
		SubscribePresence: true,
		ReadDelayMin:      30 * time.Second,
		ReadDelayMax:      time.Minute,
	}
}

func randomDuration(min, max time.Duration) time.Duration {
	if min <= 0 && max <= 0 {
		return 0
	}
	if max <= min {
		return min
	}
	return min + time.Duration(rand.Int63n(int64(max-min)+1))
}

func sleepWithContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (cli *Client) sleepHumanDelay(ctx context.Context, label string, min, max time.Duration) error {
	delay := randomDuration(min, max)
	if delay <= 0 {
		return nil
	}
	cli.Log.Debugf("模拟真人行为延迟 %s: %s", label, delay)
	return sleepWithContext(ctx, delay)
}

func (cli *Client) runPreSendHumanBehavior(ctx context.Context, to types.JID, cfg *HumanBehaviorConfig) error {
	if cfg.SendOnline {
		cli.Log.Debugf("模拟真人行为：发送在线状态")
		if err := cli.SendPresence(ctx, types.PresenceAvailable); err != nil {
			cli.Log.Debugf("给 %s 发送 DM 前发送在线状态失败: %v", to, err)
		}
	}
	if cfg.SendOfflineAfter {
		delay := randomDuration(cfg.OfflineAfterMin, cfg.OfflineAfterMax)
		go func() {
			if err := sleepWithContext(context.WithoutCancel(ctx), delay); err != nil {
				return
			}
			if cli != nil && cli.IsConnected() {
				cli.Log.Debugf("模拟真人行为：发送离线状态")
				if err := cli.SendPresence(context.WithoutCancel(ctx), types.PresenceUnavailable); err != nil {
					cli.Log.Debugf("给 %s 发送 DM 后发送离线状态失败: %v", to, err)
				}
			}
		}()
	}
	if cfg.SubscribePresence {
		cli.Log.Debugf("模拟真人行为：订阅 %s 的 presence", to)
		if err := cli.SubscribePresence(ctx, to); err != nil {
			cli.Log.Debugf("订阅 %s 的 presence 失败: %v", to, err)
		}
	}
	if cfg.SendTyping {
		cli.Log.Debugf("模拟真人行为：向 %s 发送输入中状态", to)
		if err := cli.SendChatPresence(ctx, to, types.ChatPresenceComposing, types.ChatPresenceMediaText); err != nil {
			cli.Log.Debugf("向 %s 发送输入中状态失败: %v", to, err)
		}
		if err := cli.sleepHumanDelay(ctx, "输入中", cfg.TypingDelayMin, cfg.TypingDelayMax); err != nil {
			return err
		}
		cli.Log.Debugf("模拟真人行为：向 %s 发送暂停输入状态", to)
		if err := cli.SendChatPresence(ctx, to, types.ChatPresencePaused, types.ChatPresenceMediaText); err != nil {
			cli.Log.Debugf("向 %s 发送暂停输入状态失败: %v", to, err)
		}
	}
	return cli.sleepHumanDelay(ctx, "发送消息前", cfg.SendDelayMin, cfg.SendDelayMax)
}

func (cli *Client) maybeAutoReadMessage(ctx context.Context, evt *events.Message) {
	cfg := cli.AutoReceipt
	if evt == nil {
		return
	}
	if !cfg.Enabled {
		cli.Log.Debugf("自动回执未启用，跳过消息 %s", evt.Info.ID)
		return
	}
	if evt.Info.IsFromMe {
		cli.Log.Debugf("消息 %s 是自己发送的，跳过自动回执", evt.Info.ID)
		return
	}
	if evt.Info.IsGroup && !cfg.IncludeGroups {
		cli.Log.Debugf("消息 %s 是群消息且 IncludeGroups=false，跳过自动回执", evt.Info.ID)
		return
	}
	if !cfg.SendRead && !cfg.SubscribePresence {
		cli.Log.Debugf("消息 %s 的自动回执配置没有开启已读或 presence 订阅，跳过", evt.Info.ID)
		return
	}
	cli.Log.Debugf("消息 %s 已加入自动回执流程，chat=%s sender=%s", evt.Info.ID, evt.Info.Chat, evt.Info.Sender)
	go cli.autoReadMessage(context.WithoutCancel(ctx), cfg, evt)
}

func (cli *Client) autoReadMessage(ctx context.Context, cfg AutoReceiptConfig, evt *events.Message) {
	info := evt.Info
	if cfg.SubscribePresence && !info.Chat.IsEmpty() && !info.IsGroup {
		cli.Log.Debugf("自动回执：准备订阅 %s 的 presence", info.Chat)
		if err := cli.SubscribePresence(ctx, info.Chat); err != nil {
			cli.Log.Debugf("自动回执：订阅 %s 的 presence 失败: %v", info.Chat, err)
		} else {
			cli.Log.Debugf("自动回执：订阅 %s 的 presence 成功", info.Chat)
		}
	}
	if !cfg.SendRead {
		return
	}
	delay := randomDuration(cfg.ReadDelayMin, cfg.ReadDelayMax)
	if delay > 0 {
		cli.Log.Debugf("自动回执：消息 %s 将在 %s 后发送已读", info.ID, delay)
	}
	if err := sleepWithContext(ctx, delay); err != nil {
		cli.Log.Debugf("自动回执：消息 %s 等待发送已读时被取消: %v", info.ID, err)
		return
	}
	cli.Log.Debugf("自动回执：准备标记消息 %s 为已读，chat=%s sender=%s", info.ID, info.Chat, info.Sender)
	if err := cli.MarkRead(ctx, []types.MessageID{info.ID}, time.Now(), info.Chat, info.Sender); err != nil {
		cli.Log.Debugf("自动回执：标记消息 %s 为已读失败: %v", info.ID, err)
	} else {
		cli.Log.Debugf("自动回执：标记消息 %s 为已读成功", info.ID)
	}
}
