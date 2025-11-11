package jwt_signer

import (
	"fmt"
	"net/http"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

func init() {
	caddy.RegisterModule(&JwtSigner{})
	httpcaddyfile.RegisterHandlerDirective("jwt_signer", parseCaddyfile)
	httpcaddyfile.RegisterDirectiveOrder("jwt_signer", httpcaddyfile.Before, "redir")
}

type JwtSigner struct {
	Dur    string `json:"duration"`
	Secret string `json:"secret"`
	Claims jwt.MapClaims
	l      *zap.Logger
}

func (s *JwtSigner) Provision(ctx caddy.Context) error {
	s.l = ctx.Logger()

	s.l.Debug("Provisioned", zap.String("duration", s.Dur), zap.Any("claims", s.Claims))

	return nil
}

func (s *JwtSigner) Validate() error {
	vals := map[string]string{
		"duration": s.Dur,
		"secret":   s.Secret,
	}

	for key, val := range vals {
		if val == "" {
			return fmt.Errorf("missing required parameter: %s", key)
		}
	}

	return nil
}

func (s *JwtSigner) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	s.l.Debug("Run", zap.String("path", r.URL.Path), zap.String("query", r.URL.RawQuery))

	repl := r.Context().Value(caddy.ReplacerCtxKey).(*caddy.Replacer)
	if repl == nil {
		return fmt.Errorf("no replacer found in context")
	}

	durStr, secret := repl.ReplaceAll(s.Dur, ""), repl.ReplaceAll(s.Secret, "")

	toValidate := map[string]string{
		"dur":    durStr,
		"secret": secret,
	}

	for key, val := range toValidate {
		if val == "" {
			return fmt.Errorf("required parameter empty after replacements: %s", key)
		}
	}

	dur, err := time.ParseDuration(durStr)
	if err != nil {
		return fmt.Errorf("invalid duration: %s", durStr)
	}

	s.l.Debug("Parsed duration", zap.String("as_str", durStr), zap.Float64("seconds", dur.Seconds()))

	cs := jwt.MapClaims{}
	fillClaims(s.Claims, repl, s.l, &cs)

	now := time.Now()
	cs["iat"] = now.Unix()
	cs["exp"] = now.Add(dur).Unix()

	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, cs)

	tosStr, err := tok.SignedString([]byte(secret))
	if err != nil {
		return err
	}

	repl.Set("http.jwt_signer.digest_str", tosStr)

	return next.ServeHTTP(w, r)
}

func fillClaims(pat jwt.MapClaims, repl *caddy.Replacer, l *zap.Logger, claims *jwt.MapClaims) {
	cs := jwt.MapClaims{}

	for k, v := range pat {
		switch val := v.(type) {
		case string:
			valExpanded := repl.ReplaceAll(val, "")
			l.Debug("String key expanded", zap.String("key", k), zap.String("value", valExpanded), zap.String("value_config", val))
			if valExpanded != "" {
				cs[k] = valExpanded
			}
		case map[string]any:
			nested := jwt.MapClaims(nil)
			l.Debug("Descending into nested map", zap.String("key", k))
			fillClaims(val, repl, l, &nested)
			if nested != nil {
				cs[k] = nested
			}
		default:
			l.Debug("Set value of non-obvious type", zap.String("key", k), zap.String("type", fmt.Sprintf("%T", val)))
			cs[k] = v
		}
	}

	l.Debug("Finalize current map", zap.Int("length", len(cs)))

	if len(cs) > 0 {
		*claims = cs
	}
}

func (*JwtSigner) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.jwt_signer",
		New: func() caddy.Module { return new(JwtSigner) },
	}
}

func (s *JwtSigner) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	d.Next() // consume directive name

	if !d.Args(&s.Dur, &s.Secret) {
		return d.ArgErr()
	}

	if d.NextArg() {
		return d.ArgErr()
	}

	return parseClaimsCaddyfile(d, &s.Claims)
}

func parseClaimsCaddyfile(d *caddyfile.Dispenser, claims *jwt.MapClaims) error {
	cs := jwt.MapClaims{}

	for nesting := d.Nesting(); d.NextBlock(nesting); {
		key := d.Val()
		if key == "" {
			return fmt.Errorf("malformed claims: no key found")
		}

		var val string
		if d.Args(&val) {
			if val == "" {
				return fmt.Errorf("malformed claim %s: value is empty", key)
			}

			if d.NextArg() {
				return d.Errf("too many arguments after key: %s", key)
			}

			cs[key] = val
			continue
		}

		nested := jwt.MapClaims(nil)
		if err := parseClaimsCaddyfile(d, &nested); err != nil {
			return d.Errf("nested under key %s: %w", key, err)
		}

		if nested != nil {
			cs[key] = nested
			continue
		}

		return d.Errf("mailformed claim %s: no value", key)
	}

	if len(cs) > 0 {
		*claims = cs
	}

	return nil
}

func parseCaddyfile(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	s := JwtSigner{}
	err := s.UnmarshalCaddyfile(h.Dispenser)
	return &s, err
}

var (
	_ caddy.Provisioner           = (*JwtSigner)(nil)
	_ caddy.Validator             = (*JwtSigner)(nil)
	_ caddyhttp.MiddlewareHandler = (*JwtSigner)(nil)
	_ caddyfile.Unmarshaler       = (*JwtSigner)(nil)
)
