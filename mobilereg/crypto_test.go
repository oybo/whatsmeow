package mobilereg

import (
	"strings"
	"testing"
)

func TestParamOrderHasNoDuplicates(t *testing.T) {
	seen := make(map[string]struct{}, len(paramOrder))
	for _, key := range paramOrder {
		if _, ok := seen[key]; ok {
			t.Fatalf("duplicate parameter order key %q", key)
		}
		seen[key] = struct{}{}
	}
}

func TestCodeParamsUseAndroidObservedOrder(t *testing.T) {
	params := Params{
		"reason":             "",
		"method":             "sms",
		"e_regid":            "FjBoug",
		"e_skey_sig":         "sig",
		"e_ident":            "ident",
		"e_skey_val":         "skey",
		"rc":                 "0",
		"pid":                "0",
		"in":                 "53085444",
		"expid":              "expid",
		"mistyped":           "0",
		"client_metrics":     "{}",
		"lc":                 "US",
		"authkey":            "auth",
		"simnum":             "1",
		"hasinrc":            "0",
		"hasav":              "0",
		"token":              "/DU9PZ+wyR95OBdD2esLWsFC2s4=",
		"lg":                 "en",
		"e_skey_id":          "AAAB",
		"fdid":               "ac481944-d5e4-4284-9135-d1d16449a51a",
		"e_keytype":          "BQ",
		"network_radio_type": "1",
		"cc":                 "852",
	}
	query, err := buildOrderedQuery(params, true)
	if err != nil {
		t.Fatal(err)
	}

	gotKeys := make([]string, 0, len(params))
	for _, part := range strings.Split(query, "&") {
		key, _, _ := strings.Cut(part, "=")
		gotKeys = append(gotKeys, key)
	}
	wantKeys := []string{
		"reason",
		"method",
		"e_regid",
		"e_skey_sig",
		"e_ident",
		"e_skey_val",
		"rc",
		"pid",
		"in",
		"expid",
		"mistyped",
		"client_metrics",
		"lc",
		"authkey",
		"simnum",
		"hasinrc",
		"hasav",
		"token",
		"lg",
		"e_skey_id",
		"fdid",
		"e_keytype",
		"network_radio_type",
		"cc",
	}
	if strings.Join(gotKeys, ",") != strings.Join(wantKeys, ",") {
		t.Fatalf("unexpected order:\ngot  %v\nwant %v", gotKeys, wantKeys)
	}
}
