package desktop

import "golang.org/x/sys/windows/registry"

// prefersDark reads the "app mode" setting of Windows (Settings > Colours).
// Any error means light, the Windows default.
func prefersDark() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	light, _, err := k.GetIntegerValue("AppsUseLightTheme")
	return err == nil && light == 0
}
