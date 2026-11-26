package config

import (
	"errors"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/rest"
	"net/url"
	"os"
	"strings"
)

type Config struct {
	AllowPublicRegistration bool   `json:",default=false"`
	SiteDir                 string `json:",optional"`
	AdminDir                string `json:",optional"`
	AllowPrivateProviders   bool   `json:",default=false"`
	StorageQuotaBytes       int64  `json:",default=209715200"`
	TrashDays               int    `json:",default=30"`
	rest.RestConf
	MySQLDSN       string
	JWTSecret      string
	StorageDir     string
	AllowedOrigins []string
	CookieSecure   bool `json:",default=true"`
	AccessMinutes  int  `json:",default=15"`
	RefreshDays    int  `json:",default=7"`
}

func Load(file string) (Config, error) {
	var c Config
	if e := conf.Load(file, &c, conf.UseEnv()); e != nil {
		return c, e
	}
	if v := os.Getenv("DESIGN_MYSQL_DSN"); v != "" {
		c.MySQLDSN = v
	}
	if v := os.Getenv("DESIGN_JWT_SECRET"); v != "" {
		c.JWTSecret = v
	}
	if c.MaxBytes < 21*1024*1024 {
		c.MaxBytes = 21 * 1024 * 1024
	}
	if c.Timeout < 60000 {
		c.Timeout = 60000
	}
	return c, c.Validate()
}
func (c Config) Validate() error {
	if c.StorageQuotaBytes < 0 || c.TrashDays < 0 || c.TrashDays > 365 {
		return errors.New("invalid storage quota or trash retention")
	}
	if c.MySQLDSN == "" || strings.Contains(c.MySQLDSN, "${") {
		return errors.New("set DESIGN_MYSQL_DSN")
	}
	if len(c.JWTSecret) < 32 || strings.Contains(c.JWTSecret, "${") || strings.Contains(c.JWTSecret, "CHANGE_ME") {
		return errors.New("DESIGN_JWT_SECRET must be a random secret of at least 32 characters")
	}
	if c.StorageDir == "" {
		return errors.New("StorageDir is required")
	}
	if c.AccessMinutes < 1 || c.AccessMinutes > 60 || c.RefreshDays < 1 || c.RefreshDays > 30 {
		return errors.New("invalid session lifetimes")
	}
	if len(c.AllowedOrigins) == 0 {
		return errors.New("AllowedOrigins is required")
	}
	for _, origin := range c.AllowedOrigins {
		u, e := url.Parse(origin)
		if e != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
			return errors.New("AllowedOrigins must contain exact HTTP(S) origins without paths")
		}
	}
	return nil
}
