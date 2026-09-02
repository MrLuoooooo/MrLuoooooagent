package mask

import (
	"strings"
	"testing"
)

func TestMaskSensitive_Phone(t *testing.T) {
	got := MaskSensitive("联系我 13812345678 处理")
	if !strings.Contains(got, "138****5678") {
		t.Errorf("phone not masked: %q", got)
	}
}

func TestMaskSensitive_Email(t *testing.T) {
	got := MaskSensitive("邮箱 zhangsan@example.com 收件")
	if !strings.Contains(got, "z***@example.com") {
		t.Errorf("email not masked: %q", got)
	}
}

func TestMaskSensitive_IDCard(t *testing.T) {
	got := MaskSensitive("身份证 11010119900307867X 登记")
	if !strings.Contains(got, "110101********867X") {
		t.Errorf("idcard not masked: %q", got)
	}
}

func TestMaskSensitive_SecretKey(t *testing.T) {
	got := MaskSensitive("key: sk-abcdefghij1234567890")
	if strings.Contains(got, "sk-abcdefghij1234567890") {
		t.Errorf("secret key not masked: %q", got)
	}
	if !strings.Contains(got, "sk-ab") {
		t.Errorf("secret prefix should remain: %q", got)
	}
}

func TestMaskSensitive_PlainUntouched(t *testing.T) {
	plain := `{"code": "sh600519", "limit": 20}`
	if got := MaskSensitive(plain); got != plain {
		t.Errorf("plain args should be untouched, got: %q", got)
	}
}

func TestMaskSensitive_Empty(t *testing.T) {
	if got := MaskSensitive(""); got != "" {
		t.Errorf("empty should stay empty, got %q", got)
	}
}

func TestMaskConfigValues_EnvFile(t *testing.T) {
	env := "GOAGENT_MODEL_PROVIDER_CLOUD_API_KEY=sk-real-secret-123\nMYSQL_PASSWORD=hunter2secret\nLOG_LEVEL=info\n"
	got := MaskConfigValues(env)
	if strings.Contains(got, "sk-real-secret-123") || strings.Contains(got, "hunter2secret") {
		t.Errorf("secrets leaked: %q", got)
	}
	if !strings.Contains(got, "LOG_LEVEL=info") {
		t.Errorf("non-sensitive lines should stay: %q", got)
	}
	if !strings.Contains(got, "****") {
		t.Errorf("sensitive values should be masked: %q", got)
	}
}
