package auth

type ProviderSpec struct {
	AuthURL     string   
	TokenURL    string   
	UserInfoURL string   
	Scopes      []string 
}

var Registry = map[string]ProviderSpec{
	"google": {
		AuthURL:     "https://accounts.google.com/o/oauth2/v2/auth",
		TokenURL:    "https://oauth2.googleapis.com/token",
		UserInfoURL: "https://www.googleapis.com/oauth2/v2/userinfo",
		Scopes:      []string{"email", "profile", "openid"},
	},
	"github": {
		AuthURL:     "https://github.com/login/oauth/authorize",
		TokenURL:    "https://github.com/login/oauth/access_token",
		UserInfoURL: "https://api.github.com/user",
		Scopes:      []string{"user:email"},
	},
	"microsoft": {
		AuthURL:     "https://login.microsoftonline.com/common/oauth2/v2.0/authorize",
		TokenURL:    "https://login.microsoftonline.com/common/oauth2/v2.0/token",
		UserInfoURL: "https://graph.microsoft.com/v1.0/me",
		Scopes:      []string{"openid", "profile", "email"},
	},
}
