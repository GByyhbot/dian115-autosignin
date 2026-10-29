package core

import (
	"net/url"
	"strings"
)

// SitePreset 描述一个内置站点预设（对应 autosignin 的一个专用处理器或通用 NexusPHP 小站）。
type SitePreset struct {
	Key       string
	Name      string
	Domain    string
	Contains  bool   // 域名匹配方式：true 为包含匹配（mteam/yema 风格），false 为精确匹配
	Framework string // 站点框架：nexusphp（通用 attendance 签到）；专用处理器可留空
	Homepage  string // 完整站点首页 URL（选中后自动填入配置）
}

// BuiltinPresets 常用大站（专用处理器）；ExtraNexusPresets 为通用 NexusPHP 小站。
var BuiltinPresets = []SitePreset{
	{Key: "52pt", Name: "52PT", Domain: "52pt.site", Framework: "nexusphp", Homepage: "https://52pt.site/"},
	{Key: "btschool", Name: "学校（BTSchool）", Domain: "pt.btschool.club", Framework: "nexusphp", Homepage: "https://pt.btschool.club/"},
	{Key: "chdbits", Name: "彩虹岛（CHDBits）", Domain: "ptchdbits.co", Framework: "nexusphp", Homepage: "https://ptchdbits.co/"},
	{Key: "haidan", Name: "海胆之家（HaiDan）", Domain: "haidan.cc", Framework: "nexusphp", Homepage: "https://haidan.cc/"},
	{Key: "hares", Name: "白兔（Hares）", Domain: "club.hares.top", Framework: "nexusphp", Homepage: "https://club.hares.top/"},
	{Key: "hdarea", Name: "好大（HDArea）", Domain: "hdarea.club", Framework: "nexusphp", Homepage: "https://hdarea.club/"},
	{Key: "hdchina", Name: "瓷器（HDChina）", Domain: "hdchina.org", Framework: "nexusphp", Homepage: "https://hdchina.org/"},
	{Key: "hdcity", Name: "城市（HDCity）", Domain: "hdcity.city", Framework: "nexusphp", Homepage: "https://hdcity.city/"},
	{Key: "hdsky", Name: "天空（HDSky）", Domain: "hdsky.me", Framework: "nexusphp", Homepage: "https://hdsky.me/"},
	{Key: "hdupt", Name: "HDUPT", Domain: "pt.hdupt.com", Framework: "nexusphp", Homepage: "https://pt.hdupt.com/"},
	{Key: "mteam", Name: "馒头（M-Team）", Domain: "m-team", Contains: true},
	{Key: "nexushd", Name: "NexusHD", Domain: "v6.nexushd.org", Framework: "nexusphp", Homepage: "https://v6.nexushd.org/"},
	{Key: "opencd", Name: "皇后（OpenCD）", Domain: "open.cd", Framework: "nexusphp", Homepage: "https://open.cd/"},
	{Key: "pterclub", Name: "猫（PTerClub）", Domain: "pterclub.com", Framework: "nexusphp", Homepage: "https://pterclub.com/"},
	{Key: "pttime", Name: "PT时间（PTTime）", Domain: "pttime.org", Framework: "nexusphp", Homepage: "https://pttime.org/"},
	{Key: "ptzone", Name: "PTZONE", Domain: "ptzone.xyz", Framework: "nexusphp", Homepage: "https://ptzone.xyz/"},
	{Key: "rousi", Name: "Rousi Pro", Domain: "rousi.pro"},
	{Key: "tjupt", Name: "北洋（TJUPT）", Domain: "tjupt.org", Framework: "nexusphp", Homepage: "https://tjupt.org/"},
	{Key: "ttg", Name: "TTG", Domain: "totheglory.im"},
	{Key: "u2", Name: "U2", Domain: "u2.dmhy.org", Framework: "nexusphp", Homepage: "https://u2.dmhy.org/"},
	{Key: "yema", Name: "YemaPT", Domain: "yemapt.org", Contains: true},
	{Key: "zhuque", Name: "朱雀（ZHUQUE）", Domain: "zhuque.in", Framework: "nexusphp", Homepage: "https://zhuque.in/"},
}

// AllPresets 返回全部内置预设（大站 + NexusPHP 小站）。
func AllPresets() []SitePreset {
	merged := make([]SitePreset, 0, len(BuiltinPresets)+len(ExtraNexusPresets))
	merged = append(merged, BuiltinPresets...)
	merged = append(merged, ExtraNexusPresets...)
	return merged
}

// PresetByKey 返回预设与是否存在标记。
func PresetByKey(key string) (SitePreset, bool) {
	for _, p := range AllPresets() {
		if p.Key == key {
			return p, true
		}
	}
	return SitePreset{}, false
}

// HostOf 提取 URL 的规范化主机名（去 scheme、去前导 www.、小写）。
func HostOf(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	host := u.Hostname()
	host = strings.TrimPrefix(strings.ToLower(host), "www.")
	return host
}

// MatchPreset 按 URL 匹配内置预设，返回键；无匹配返回 ""。
func MatchPreset(raw string) string {
	host := HostOf(raw)
	if host == "" {
		return ""
	}
	for _, p := range AllPresets() {
		if p.Contains {
			if strings.Contains(host, p.Domain) {
				return p.Key
			}
			continue
		}
		if host == p.Domain || host == strings.TrimPrefix(p.Domain, "www.") {
			return p.Key
		}
	}
	return ""
}

// MatchPresetExact 供 UI 校验站点 URL 与所选预设一致。
func MatchPresetExact(raw, key string) bool {
	p, ok := PresetByKey(key)
	if !ok {
		return true // 自定义站点不校验
	}
	host := HostOf(raw)
	if p.Contains {
		return strings.Contains(host, p.Domain)
	}
	return host == p.Domain || host == strings.TrimPrefix(p.Domain, "www.")
}
