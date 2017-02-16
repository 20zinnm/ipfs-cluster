package config

import "testing"

func TestDefaultConfig(t *testing.T) {
	_, err := NewDefaultConfig()
	if err != nil {
		t.Fatal(err)
	}
}

func TestConfigToJSON(t *testing.T) {
	cfg, err := NewDefaultConfig()
	if err != nil {
		t.Fatal(err)
	}
	_, err = cfg.ToJSONConfig()
	if err != nil {
		t.Error(err)
	}
}

func TestConfigToConfig(t *testing.T) {
	cfg, _ := NewDefaultConfig()
	j, _ := cfg.ToJSONConfig()
	_, err := j.ToConfig()
	if err != nil {
		t.Error(err)
	}

	j.ID = "abc"
	_, err = j.ToConfig()
	if err == nil {
		t.Error("expected error decoding ID")
	}

	j, _ = cfg.ToJSONConfig()
	j.PrivateKey = "abc"
	_, err = j.ToConfig()
	if err == nil {
		t.Error("expected error parsing private key")
	}

	j, _ = cfg.ToJSONConfig()
	j.ClusterListenMultiaddress = "abc"
	_, err = j.ToConfig()
	if err == nil {
		t.Error("expected error parsing cluster_listen_multiaddress")
	}

	j, _ = cfg.ToJSONConfig()
	j.APIListenMultiaddress = "abc"
	_, err = j.ToConfig()
	if err == nil {
		t.Error("expected error parsing api_listen_multiaddress")
	}

	j, _ = cfg.ToJSONConfig()
	j.IPFSProxyListenMultiaddress = "abc"
	_, err = j.ToConfig()
	if err == nil {
		t.Error("expected error parsing ipfs_proxy_listen_multiaddress")
	}

	j, _ = cfg.ToJSONConfig()
	j.IPFSNodeMultiaddress = "abc"
	_, err = j.ToConfig()
	if err == nil {
		t.Error("expected error parsing ipfs_node_multiaddress")
	}

	j, _ = cfg.ToJSONConfig()
	j.Bootstrap = []string{"abc"}
	_, err = j.ToConfig()
	if err == nil {
		t.Error("expected error parsing Bootstrap")
	}
}
