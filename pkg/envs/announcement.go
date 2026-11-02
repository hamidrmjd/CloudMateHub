package envs

import (
	"fmt"
	"strings"
)

const announcementFormat = `
******************************************************************
*                                                                *
*%s*
*                                                                *
* CloudMateHub centralizes SSH access to your Linux devices.      *
* Connect from a browser, mobile device, or standard SSH client.   *
* Your infrastructure, your control.                              *
*                                                                *
* For assistance, contact the system administrator.               *
*                                                                *
******************************************************************
`

// AnnouncementFor returns the SSH connection banner for the given edition.
func AnnouncementFor(edition Edition) string {
	if edition == "" {
		edition = Community
	}

	e := string(edition)
	title := "Welcome to CloudMateHub " + strings.ToUpper(e[:1]) + e[1:] + "!"
	pad := 64 - len(title)

	return fmt.Sprintf(announcementFormat, fmt.Sprintf("%*s%s%*s", pad/2, "", title, pad-pad/2, ""))
}
