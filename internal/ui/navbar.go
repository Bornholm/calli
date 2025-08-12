package ui

// MenuItem represents an item in the navigation menu
type NavbarItem struct {
	Label    string
	URL      string
	Icon     string
	Position string // "left" or "right"
}

type NavbarTemplateData struct {
	Username    string
	NavbarItems []NavbarItem
}

var NavbarItemLogout = NavbarItem{
	Label:    "Logout",
	URL:      "/auth/logout",
	Icon:     "fa-sign-out-alt",
	Position: "right",
}
