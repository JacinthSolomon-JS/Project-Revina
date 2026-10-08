//go:build darwin

package hardening

func init() {
	register("gatekeeper", buildGatekeeper)
	register("remote-login", buildRemoteLogin)
	register("sshd-password-auth", buildSSHD)
}
