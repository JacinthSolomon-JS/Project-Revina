//go:build linux

package hardening

func init() {
	register("sysctl-secure", buildSysctl)
	register("ufw-enable", buildUFW)
	register("sshd-password-auth", buildSSHD)
}
