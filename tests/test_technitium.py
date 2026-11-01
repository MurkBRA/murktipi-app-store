import pytest

EXPECTED_TECHNITIUM_OPTIONS = [
    # Forwarders & Protocols
    "DNS_SERVER_FORWARDERS",
    "DNS_SERVER_FORWARDER_PROTOCOL",
    "DNS_SERVER_PREFER_IPV6",
    # Blocking
    "DNS_SERVER_ENABLE_BLOCKING",
    "DNS_SERVER_ALLOW_TXT_BLOCKING_REPORT",
    "DNS_SERVER_BLOCK_LIST_URLS",
    # Web Service
    "DNS_SERVER_WEB_SERVICE_HTTP_PORT",
    "DNS_SERVER_WEB_SERVICE_HTTPS_PORT",
    "DNS_SERVER_WEB_SERVICE_ENABLE_HTTPS",
    "DNS_SERVER_WEB_SERVICE_USE_SELF_SIGNED_CERT",
    "DNS_SERVER_WEB_SERVICE_TLS_CERTIFICATE_PATH",
    "DNS_SERVER_WEB_SERVICE_TLS_CERTIFICATE_PASSWORD",
    "DNS_SERVER_WEB_SERVICE_HTTP_TO_TLS_REDIRECT",
    "DNS_SERVER_WEB_SERVICE_REVERSE_PROXY_ADDRESSES",
    "DNS_SERVER_OPTIONAL_PROTOCOL_DNS_OVER_HTTP",
    # Recursion ACL
    "DNS_SERVER_RECURSION_NETWORK_ACL",
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
    "DNS_SERVER_SSO_METADATA_ADDRESS",
    "DNS_SERVER_SSO_SCOPES",
    "DNS_SERVER_SSO_ALLOW_SIGNUP",
    "DNS_SERVER_SSO_GROUP_MAP",
    # LDAP / AD
    "DNS_SERVER_LDAP_ENABLED",
    "DNS_SERVER_LDAP_SERVER",
    "DNS_SERVER_LDAP_PORT",
    "DNS_SERVER_LDAP_SSL_OPTION",
    "DNS_SERVER_LDAP_BIND_USERNAME",
    "DNS_SERVER_LDAP_BIND_PASSWORD",
    "DNS_SERVER_LDAP_SEARCH_BASE",
    "DNS_SERVER_LDAP_USER_SEARCH_FILTER",
    "DNS_SERVER_LDAP_GROUP_ATTRIBUTE",
    "DNS_SERVER_LDAP_ALLOW_SIGNUP",
    "DNS_SERVER_LDAP_GROUP_MAP",
]

def test_technitium_config_defaults(get_app_config):
    config = get_app_config("technitium")
    form_fields = {f["env_variable"]: f for f in config.get("form_fields", [])}

    # Verify the 3 standard defaults
    assert "DNS_SERVER_DOMAIN" in form_fields
    assert form_fields["DNS_SERVER_DOMAIN"]["default"] == "technitium-dns"

    assert "DNS_SERVER_RECURSION" in form_fields
    assert form_fields["DNS_SERVER_RECURSION"]["default"] == "Allow"

    assert "DNS_SERVER_ADMIN_PASSWORD" in form_fields

    # Verify that optional extra slots have NO default values
    for i in range(1, 6):
        var_name = f"EXTRA_VAR_{i}_NAME"
        var_val = f"EXTRA_VAR_{i}_VALUE"
        assert var_name in form_fields
        assert var_val in form_fields
        assert form_fields[var_name]["default"] == "", f"{var_name} must not have a default value"
        assert form_fields[var_val]["default"] == "", f"{var_val} must not have a default value"
        assert form_fields[var_name]["required"] is False
        assert form_fields[var_val]["required"] is False

def test_technitium_dropdown_has_all_docker_documentation_variables(get_app_config):
    config = get_app_config("technitium")
    form_fields = {f["env_variable"]: f for f in config.get("form_fields", [])}

    slot1 = form_fields["EXTRA_VAR_1_NAME"]
    assert "options" in slot1
    option_values = {opt["value"] for opt in slot1["options"]}

    for expected_var in EXPECTED_TECHNITIUM_OPTIONS:
        assert expected_var in option_values, (
            f"Technitium dropdown list missing official variable '{expected_var}'"
        )

def test_technitium_compose_structure(get_app_compose):
    compose = get_app_compose("technitium")
    services = compose["services"]

    # Service name must be technitium-dns
    assert "technitium-dns" in services
    service = services["technitium-dns"]

    # Check container_name, image, restart
    assert service.get("container_name") == "technitium-dns"
    assert service.get("image") == "technitium/dns-server:latest"
    assert service.get("restart") == "unless-stopped"

    # Check standard ports
    ports = [str(p) for p in service.get("ports", [])]
    assert any("53:53/tcp" in p for p in ports), "Missing port 53:53/tcp"
    assert any("53:53/udp" in p for p in ports), "Missing port 53:53/udp"
    assert any("5380" in p for p in ports), "Missing port 5380"

def test_technitium_entrypoint_prunes_empty_vars(get_app_compose):
    compose = get_app_compose("technitium")
    service = compose["services"]["technitium-dns"]
    assert "entrypoint" in service
    entrypoint_str = str(service["entrypoint"])
    assert "EXTRA_VAR_" in entrypoint_str
    assert "unset" in entrypoint_str
    assert "DnsServerApp.dll" in entrypoint_str
