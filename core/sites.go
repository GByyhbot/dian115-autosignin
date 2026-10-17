package core

import (
	"net/url"
	"strings"
)

// SitePreset 描述一个内置站点预设（对应 autosignin 的一个专用处理器）。
type SitePreset struct {
	Key     string
	Name    string
	Domain  string
	Contains bool // 域名匹配方式：true 为包含匹配（mteam/yema 风格），false 为精确匹配
}

// BuiltinPresets 全部内置站点预设（与 autosignin 3.0.3 sites 目录一致，共 22 个）。
var BuiltinPresets = []SitePreset{
	{Key: "52pt", Name: "52PT", Domain: "52pt.site"},
	{Key: "btschool", Name: "学校（BTSchool）", Domain: "pt.btschool.club"},
	{Key: "chdbits", Name: "彩虹岛（CHDBits）", Domain: "ptchdbits.co"},
	{Key: "haidan", Name: "海胆之家（HaiDan）", Domain: "haidan.cc"},
	{Key: "hares", Name: "白兔（Hares）", Domain: "club.hares.top"},
	{Key: "hdarea", Name: "好大（HDArea）", Domain: "hdarea.club"},
	{Key: "hdchina", Name: "瓷器（HDChina）", Domain: "hdchina.org"},
	{Key: "hdcity", Name: "城市（HDCity）", Domain: "hdcity.city"},
	{Key: "hdsky", Name: "天空（HDSky）", Domain: "hdsky.me"},
	{Key: "hdupt", Name: "HDUPT", Domain: "pt.hdupt.com"},
	{Key: "mteam", Name: "馒头（M-Team）", Domain: "m-team", Contains: true},
	{Key: "nexushd", Name: "NexusHD", Domain: "v6.nexushd.org"},
	{Key: "opencd", Name: "皇后（OpenCD）", Domain: "open.cd"},
	{Key: "pterclub", Name: "猫（PTerClub）", Domain: "pterclub.com"},
	{Key: "pttime", Name: "PT时间（PTTime）", Domain: "pttime.org"},
	{Key: "ptzone", Name: "PTZONE", Domain: "ptzone.xyz"},
	{Key: "rousi", Name: "Rousi Pro", Domain: "rousi.pro"},
	{Key: "tjupt", Name: "北洋（TJUPT）", Domain: "tjupt.org"},
	{Key: "ttg", Name: "TTG", Domain: "totheglory.im"},
	{Key: "u2", Name: "U2", Domain: "u2.dmhy.org"},
	{Key: "yema", Name: "YemaPT", Domain: "yemapt.org"},
	{Key: "zhuque", Name: "朱雀（ZHUQUE）", Domain: "zhuque.in"},
}

// PresetByKey 返回预设与是否存在标记。
func PresetByKey(key string) (SitePreset, bool) {
	for _, p := range BuiltinPresets {
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
	for _, p := range BuiltinPresets {
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
