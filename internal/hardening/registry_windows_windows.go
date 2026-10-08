//go:build windows

package hardening

func init() {
	register("disable-smbv1", buildDisableSMBv1)
	register("powershell-policy", buildPowershellPolicy)
}
