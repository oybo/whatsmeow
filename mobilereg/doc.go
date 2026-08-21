// Copyright (c) 2026
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package mobilereg implements the WhatsApp mobile registration HTTP endpoints.
//
// The package is intentionally independent from whatsmeow's Web Multidevice
// socket, pairing, and store code. It only builds and sends encrypted HTTPS
// requests for the Android-style /v2/exist, /v2/code, /v2/register, and
// /v2/security endpoints. Callers are responsible for providing Android device
// metadata, identity/pre-key material, and any token/vname values required by
// the target registration flow.
package mobilereg
