# Portier + Nginx auth_request

This is a small Go web application that can be used with Nginx `auth_request`
to protect any virtual host in Nginx with email login via [Portier]. No need
for application support, or even just to protect static files!

[portier]: http://portier.io

## Installing

Build nginx-auth with: (no git checkout needed)

```sh
go get github.com/portier/nginx-auth
```

The above command has no output if successful, and you'll find a binary at
`~/go/bin/nginx-auth`.

(In the future, we may create actual releases and offer ready-made binaries for
download, but for now, this project is considered beta.)

You'll also need an Nginx install with `auth_request` support. Many
distributions enable this out-of-the-box. You can check your install by
running:

```sh
nginx -V
```

Then look for `--with-http_auth_request_module`. If that's not listed, you may
have to [build Nginx from sources](https://nginx.org/en/docs/configure.html).

## Usage

As an example, we'll start with this simple Nginx config for a site serving
static files:

```nginx
server {
	listen 80;
	server_name example.com;

	location / {
		root /var/www/example.com;
	}
}
```

### Enable auth_request

The first step is to enable `auth_request`. In the `server` block, add:

```nginx
auth_request @authcheck;

location = @authcheck {
	proxy_pass http://localhost:8081/check;
	proxy_pass_request_body off;
	proxy_set_header Content-Length "";
}
```

This creates a 'named location' `@authcheck` that proxies to the nginx-auth
`/check` route. (The default port of nginx-auth is 8081, but you can change
this with the `-listen` flag when you later start nginx-auth.)

The `auth_request @authcheck` line is the most important. For every request,
Nginx will now first send a 'subrequest' to `@authcheck`, to see if it should
be blocked or not, before actually serving your site. This subrequest is
handled in nginx-auth by checking cookies.

The extra `proxy_*` settings in `@authcheck` prevent Nginx from forwarding any
request body to nginx-auth, because it doesn't care. Only headers are used to
perform the check.

With this in place, your application is already protected, but will just show a
dull white error page, and offer no way to login.

### Add login routes

To add login pages to your site, we need to expose the login routes of
nginx-auth somewhere. You'll want to pick something that doesn't conflict with
your static files (or your real application routes). Here, we'll pick
`/_portier`, which is a pretty safe default, but remember that this is
user-visible and you can customize this:

```nginx
location /_portier {
	proxy_pass http://localhost:8081;
	auth_request off;
}
```

Notably, we disable `auth_request` for this location, because protecting the
login form would defeat its purpose.

You can now start nginx-auth. Two flags are required: `-url` and `-secret`. Set
the `-secret` flag to some random text; it is used to protect the cookie from
tampering. Set the `-url` flag to match the full URL to `/_portier` (or your
chosen path). For example:

```sh
nginx-auth \
	-secret 'this is just an example, replace it` \
	-url http://example.com/_portier
```

(Eventually, you want to run this as a background service. That's currently
something for you to figure out.)

If you visit the application now, you'll still get an error page. But, if you
navigate to `/_portier`, you should be able to complete a login. It'll then set
a cookie, redirect you back to `/`, and you should now see your site again!

### Redirecting to the login form

To present something more useful than an error page when logged out, we can
configure Nginx to serve a redirect to the login form on 403 errors:

```nginx
error_page 403 = @error403;

location @error403 {
	return 303 /_portier;
}
```

### Limiting access

Right now, anyone is allowed in, as long as they verify their email address.
The idea is probably to limit access to your site, though. You can do so by
specifying a file that contains a list of allowed emails.

```sh
nginx-auth \
	-secret 'this is just an example, replace it` \
	-url http://example.com/_portier \
	-allowlist emails.txt
```

### Complete example

See [test-app/server.conf](./test-app/server.conf) for a complete example. This
example also shows how you can send the logged in email address as a header to
an application.
