import pytest

EXPECTED_TECHNITIUM_VARS = [
    # Core & Admin
    "DNS_SERVER_DOMAIN",
    "DNS_SERVER_ADMIN_PASSWORD",
    "DNS_SERVER_ADMIN_PASSWORD_FILE",
    "DNS_SERVER_PREFER_IPV6",
    # Web Service
    "DNS_SERVER_WEB_SERVICE_LOCAL_ADDRESSES",
    "DNS_SERVER_WEB_SERVICE_HTTP_PORT",
    "DNS_SERVER_WEB_SERVICE_HTTPS_PORT",
    "DNS_SERVER_WEB_SERVICE_ENABLE_HTTPS",
    "DNS_SERVER_WEB_SERVICE_USE_SELF_SIGNED_CERT",
    "DNS_SERVER_WEB_SERVICE_TLS_CERTIFICATE_PATH",
    "DNS_SERVER_WEB_SERVICE_TLS_CERTIFICATE_PASSWORD",
    "DNS_SERVER_WEB_SERVICE_HTTP_TO_TLS_REDIRECT",
    "DNS_SERVER_WEB_SERVICE_REVERSE_PROXY_ADDRESSES",
    "DNS_SERVER_OPTIONAL_PROTOCOL_DNS_OVER_HTTP",
    # Resolution & Recursion
    "DNS_SERVER_RECURSION",
    "DNS_SERVER_RECURSION_NETWORK_ACL",
    "DNS_SERVER_RECURSION_DENIED_NETWORKS",
    "DNS_SERVER_RECURSION_ALLOWED_NETWORKS",
    # Blocking
    "DNS_SERVER_ENABLE_BLOCKING",
    "DNS_SERVER_ALLOW_TXT_BLOCKING_REPORT",
    "DNS_SERVER_BLOCK_LIST_URLS",
    # Forwarders
    "DNS_SERVER_FORWARDERS",
    "DNS_SERVER_FORWARDER_PROTOCOL",
    # Logging & Stats
    "DNS_SERVER_LOG_USING_LOCAL_TIME",
    "DNS_SERVER_LOG_FOLDER_PATH",
    "DNS_SERVER_LOG_MAX_LOG_FILE_DAYS",
    "DNS_SERVER_STATS_ENABLE_IN_MEMORY_STATS",
    "DNS_SERVER_STATS_MAX_STAT_FILE_DAYS",
    # SSO / OIDC
    "DNS_SERVER_SSO_ENABLED",
    "DNS_SERVER_SSO_AUTHORITY",
    "DNS_SERVER_SSO_CLIENT_ID",
    "DNS_SERVER_SSO_CLIENT_SECRET",
    "DNS_SERVER_SSO_CLIENT_SECRET_FILE",
    "DNS_SERVER_SSO_METADATA_ADDRESS",
    "DNS_SERVER_SSO_SCOPES",
    "DNS_SERVER_SSO_ALLOW_SIGNUP",
    "DNS_SERVER_SSO_ALLOW_SIGNUP_ONLY_FOR_MAPPED_USERS",
    "DNS_SERVER_SSO_GROUP_MAP",
    # LDAP / AD
    "DNS_SERVER_LDAP_ENABLED",
    "DNS_SERVER_LDAP_SERVER",
    "DNS_SERVER_LDAP_PORT",
    "DNS_SERVER_LDAP_SSL_OPTION",
    "DNS_SERVER_LDAP_IGNORE_SSL_ERRORS",
    "DNS_SERVER_LDAP_BIND_USERNAME",
    "DNS_SERVER_LDAP_BIND_PASSWORD",
    "DNS_SERVER_LDAP_BIND_PASSWORD_FILE",
    "DNS_SERVER_LDAP_SEARCH_BASE",
    "DNS_SERVER_LDAP_USER_SEARCH_FILTER",
    "DNS_SERVER_LDAP_GROUP_ATTRIBUTE",
    "DNS_SERVER_LDAP_ALLOW_SIGNUP",
    "DNS_SERVER_LDAP_ALLOW_SIGNUP_ONLY_FOR_MAPPED_USERS",
    "DNS_SERVER_LDAP_GROUP_MAP",
]

def test_technitium_has_all_docker_documentation_variables(get_app_config):
    config = get_app_config("technitium")
    form_fields = config.get("form_fields", [])
    registered_vars = {f["env_variable"]: f for f in form_fields}

    for expected_var in EXPECTED_TECHNITIUM_VARS:
        assert expected_var in registered_vars, (
            f"Technitium is missing expected environment variable '{expected_var}' from official docker documentation"
        )

def test_technitium_compose_maps_all_variables(get_app_compose):
    compose = get_app_compose("technitium")
    services = compose["services"]
    assert "dns-server" in services
    dns = services["dns-server"]

    env = dns.get("environment", {})
    for expected_var in EXPECTED_TECHNITIUM_VARS:
        assert expected_var in env, f"docker-compose.yml dns-server service missing environment key '{expected_var}'"

def test_technitium_ports_and_networking(get_app_compose):
    compose = get_app_compose("technitium")
    dns = compose["services"]["dns-server"]
    ports = [str(p) for p in dns.get("ports", [])]

    # Must expose standard DNS port 53 UDP and TCP
    assert any("53:53/udp" in p for p in ports), "Technitium must map 53:53/udp for DNS queries"
    assert any("53:53/tcp" in p for p in ports), "Technitium must map 53:53/tcp for DNS queries"

    # Web console
    assert any("5380" in p for p in ports), "Technitium must expose web console port 5380"

    # Must have sysctls for local port range
    assert "sysctls" in dns
    assert any("net.ipv4.ip_local_port_range" in str(s) for s in dns["sysctls"])

    # Must have healthcheck
    assert "healthcheck" in dns

def test_technitium_entrypoint_prunes_empty_vars(get_app_compose):
    compose = get_app_compose("technitium")
    dns = compose["services"]["dns-server"]
    assert "entrypoint" in dns
    entrypoint_str = str(dns["entrypoint"])
    assert "DNS_SERVER_" in entrypoint_str
    assert "unset" in entrypoint_str

