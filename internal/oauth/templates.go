package oauth

// ProviderTemplate is a built-in OAuth provider configuration template.
type ProviderTemplate struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	AuthorizeURL string   `json:"authorize_url"`
	TokenURL     string   `json:"token_url"`
	Scopes       []string `json:"scopes"`
	UsePKCE      bool     `json:"use_pkce"`
	NeedsSecret  bool     `json:"needs_secret"`
	SetupURL     string   `json:"setup_url"`
	HelpText     string   `json:"help_text"`
	CallbackURL  string   `json:"callback_url,omitempty"`
}

var templates = map[string]ProviderTemplate{
	"github": {
		ID:           "github",
		Name:         "GitHub",
		AuthorizeURL: "https://github.com/login/oauth/authorize",
		TokenURL:     "https://github.com/login/oauth/access_token",
		Scopes:       []string{"repo", "read:org", "gist", "workflow", "read:user", "user:email", "project"},
		UsePKCE:      true,
		NeedsSecret:  true,
		SetupURL:     "https://github.com/settings/developers",
		HelpText:     "Create an OAuth App under Settings > Developer settings > OAuth Apps",
	},
	"linear": {
		ID:           "linear",
		Name:         "Linear",
		AuthorizeURL: "https://linear.app/oauth/authorize",
		TokenURL:     "https://api.linear.app/oauth/token",
		Scopes:       []string{"read", "write"},
		UsePKCE:      false,
		NeedsSecret:  true,
		SetupURL:     "https://linear.app/settings/api",
		HelpText:     "Create an OAuth2 application in Linear Settings > API > OAuth2 Applications",
	},
	"clickup": {
		ID:           "clickup",
		Name:         "ClickUp",
		AuthorizeURL: "https://app.clickup.com/api",
		TokenURL:     "https://api.clickup.com/api/v2/oauth/token",
		Scopes:       []string{},
		UsePKCE:      false,
		NeedsSecret:  true,
		SetupURL:     "https://app.clickup.com/settings/integrations",
		HelpText:     "Create an app in ClickUp Settings > Integrations > ClickUp API",
	},
	"google": {
		ID:           "google",
		Name:         "Google",
		AuthorizeURL: "https://accounts.google.com/o/oauth2/v2/auth",
		TokenURL:     "https://oauth2.googleapis.com/token",
		Scopes:       []string{"openid", "email", "profile"},
		UsePKCE:      true,
		NeedsSecret:  true,
		SetupURL:     "https://console.cloud.google.com/apis/credentials",
		HelpText:     "Create OAuth 2.0 credentials in Google Cloud Console",
	},
}

// ListTemplates returns all built-in provider templates.
func ListTemplates() []ProviderTemplate {
	out := make([]ProviderTemplate, 0, len(templates))
	for _, t := range templates {
		out = append(out, t)
	}
	return out
}

// GetTemplate returns a template by ID, or nil if not found.
func GetTemplate(id string) *ProviderTemplate {
	t, ok := templates[id]
	if !ok {
		return nil
	}
	return &t
}
