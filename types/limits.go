// Copyright (c) 2026 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package types

import "go.mau.fi/util/jsontime"

type NewChatMessageCappingOTEStatus string

const (
	NewChatMessageCappingOTEStatusNotEligible          NewChatMessageCappingOTEStatus = "NOT_ELIGIBLE"
	NewChatMessageCappingOTEStatusEligible             NewChatMessageCappingOTEStatus = "ELIGIBLE"
	NewChatMessageCappingOTEStatusActiveInCurrentCycle NewChatMessageCappingOTEStatus = "ACTIVE_IN_CURRENT_CYCLE"
	NewChatMessageCappingOTEStatusExhausted            NewChatMessageCappingOTEStatus = "EXHAUSTED"
)

type NewChatMessageCappingMVStatus string

const (
	NewChatMessageCappingMVStatusNotEligible            NewChatMessageCappingMVStatus = "NOT_ELIGIBLE"
	NewChatMessageCappingMVStatusNotActive              NewChatMessageCappingMVStatus = "NOT_ACTIVE"
	NewChatMessageCappingMVStatusActive                 NewChatMessageCappingMVStatus = "ACTIVE"
	NewChatMessageCappingMVStatusActiveUpgradeAvailable NewChatMessageCappingMVStatus = "ACTIVE_UPGRADE_AVAILABLE"
)

type NewChatMessageCappingStatus string

const (
	NewChatMessageCappingStatusNone          NewChatMessageCappingStatus = "NONE"
	NewChatMessageCappingStatusFirstWarning  NewChatMessageCappingStatus = "FIRST_WARNING"
	NewChatMessageCappingStatusSecondWarning NewChatMessageCappingStatus = "SECOND_WARNING"
	NewChatMessageCappingStatusCapped        NewChatMessageCappingStatus = "CAPPED"
)

type ReachoutTimelockEnforcementType string

const (
	// 默认处罚
	ReachoutTimelockEnforcementTypeDefault ReachoutTimelockEnforcementType = "DEFAULT"
	// 商业质量分过低
	ReachoutTimelockEnforcementTypeBizQuality ReachoutTimelockEnforcementType = "BIZ_QUALITY"
	// 成人内容
	ReachoutTimelockEnforcementTypeBizCommerceViolationAdult ReachoutTimelockEnforcementType = "BIZ_COMMERCE_VIOLATION_ADULT"
	// 酒类推广
	ReachoutTimelockEnforcementTypeBizCommerceViolationAlcohol ReachoutTimelockEnforcementType = "BIZ_COMMERCE_VIOLATION_ALCOHOL"
	// 活体动物交易
	ReachoutTimelockEnforcementTypeBizCommerceViolationAnimals ReachoutTimelockEnforcementType = "BIZ_COMMERCE_VIOLATION_ANIMALS"
	// 人体组织/体液。黑市类风控
	ReachoutTimelockEnforcementTypeBizCommerceViolationBodyPartsFluids ReachoutTimelockEnforcementType = "BIZ_COMMERCE_VIOLATION_BODY_PARTS_FLUIDS"
	// 交友/约会，色情
	ReachoutTimelockEnforcementTypeBizCommerceViolationDating ReachoutTimelockEnforcementType = "BIZ_COMMERCE_VIOLATION_DATING"
	// 数字产品
	ReachoutTimelockEnforcementTypeBizCommerceViolationDigitalServicesProducts ReachoutTimelockEnforcementType = "BIZ_COMMERCE_VIOLATION_DIGITAL_SERVICES_PRODUCTS"
	// 药物
	ReachoutTimelockEnforcementTypeBizCommerceViolationDrugs ReachoutTimelockEnforcementType = "BIZ_COMMERCE_VIOLATION_DRUGS"
	// 非处方药
	ReachoutTimelockEnforcementTypeBizCommerceViolationDrugsOnlyOTC ReachoutTimelockEnforcementType = "BIZ_COMMERCE_VIOLATION_DRUGS_ONLY_OTC"
	// 博彩
	ReachoutTimelockEnforcementTypeBizCommerceViolationGambling ReachoutTimelockEnforcementType = "BIZ_COMMERCE_VIOLATION_GAMBLING"
	// 医疗
	ReachoutTimelockEnforcementTypeBizCommerceViolationHealthcare ReachoutTimelockEnforcementType = "BIZ_COMMERCE_VIOLATION_HEALTHCARE"
	// 真钱/假币
	ReachoutTimelockEnforcementTypeBizCommerceViolationRealFakeCurrency ReachoutTimelockEnforcementType = "BIZ_COMMERCE_VIOLATION_REAL_FAKE_CURRENCY"
	// 保健品
	ReachoutTimelockEnforcementTypeBizCommerceViolationSupplements ReachoutTimelockEnforcementType = "BIZ_COMMERCE_VIOLATION_SUPPLEMENTS"
	// 烟草
	ReachoutTimelockEnforcementTypeBizCommerceViolationTobacco ReachoutTimelockEnforcementType = "BIZ_COMMERCE_VIOLATION_TOBACCO"
	// 暴力内容
	ReachoutTimelockEnforcementTypeBizCommerceViolationViolentContent ReachoutTimelockEnforcementType = "BIZ_COMMERCE_VIOLATION_VIOLENT_CONTENT"
	// 武器
	ReachoutTimelockEnforcementTypeBizCommerceViolationWeapons ReachoutTimelockEnforcementType = "BIZ_COMMERCE_VIOLATION_WEAPONS"
	// 网页从设备
	ReachoutTimelockEnforcementTypeWebCompanionOnly ReachoutTimelockEnforcementType = "WEB_COMPANION_ONLY"
)

type NewChatMessageCappingInfo struct {
	TotalQuota          int                            `json:"total_quota"`
	UsedQuota           int                            `json:"used_quota"`
	CycleStartTimestamp jsontime.UnixString            `json:"cycle_start_timestamp"`
	CycleEndTimestamp   jsontime.UnixString            `json:"cycle_end_timestamp"`
	ServerSentTimestamp jsontime.UnixString            `json:"server_sent_timestamp"`
	OTEStatus           NewChatMessageCappingOTEStatus `json:"ote_status"`
	MVStatus            NewChatMessageCappingMVStatus  `json:"mv_status"`
	CappingStatus       NewChatMessageCappingStatus    `json:"capping_status"`
}

type AccountReachoutTimelock struct {
	IsActive            bool                            `json:"is_active"`
	TimeEnforcementEnds jsontime.UnixString             `json:"time_enforcement_ends"`
	EnforcementType     ReachoutTimelockEnforcementType `json:"enforcement_type"`
}
