/* WPE Kiosk — minimal fullscreen browser using WPEPlatform API */
#include <wpe/webkit.h>
#include <wpe/wpe-platform.h>
#include <gio/gio.h>
#include <json-glib/json-glib.h>
#include <stdlib.h>
#include <stdio.h>
#include <string.h>
#include <sys/statvfs.h>
#include <unistd.h>
#include <ifaddrs.h>
#include <arpa/inet.h>

static WebKitWebView *g_web_view = NULL;
static WebKitNetworkSession *g_session = NULL;

/* ---- Extension metadata ---- */

typedef struct {
    gchar *name;
    gchar *version;
    gchar *dir_path;
    gchar **scripts;
    gchar **styles;
    gboolean enabled;
} ExtensionMeta;

static GArray *g_extensions = NULL;

static void extension_meta_clear(gpointer data)
{
    ExtensionMeta *m = (ExtensionMeta *)data;
    g_free(m->name);
    g_free(m->version);
    g_free(m->dir_path);
    g_strfreev(m->scripts);
    g_strfreev(m->styles);
}

/* ---- File utilities ---- */

static gchar *read_text_file(const gchar *path)
{
    gchar *contents = NULL;
    GError *error = NULL;
    if (!g_file_get_contents(path, &contents, NULL, &error)) {
        g_warning("Cannot read file %s: %s", path, error->message);
        g_error_free(error);
        return NULL;
    }
    return contents;
}

/* ---- Extension scanning ---- */

static gboolean validate_files(const gchar *dir, gchar **files)
{
    if (!files) return TRUE;
    for (int i = 0; files[i]; i++) {
        gchar *path = g_build_filename(dir, files[i], NULL);
        gboolean exists = g_file_test(path, G_FILE_TEST_IS_REGULAR);
        g_free(path);
        if (!exists) return FALSE;
    }
    return TRUE;
}

static gchar **json_array_to_strv(JsonArray *arr)
{
    guint len = json_array_get_length(arr);
    gchar **result = g_new0(gchar *, len + 1);
    for (guint i = 0; i < len; i++)
        result[i] = g_strdup(json_array_get_string_element(arr, i));
    return result;
}

static void scan_extensions(const gchar *extensions_dir)
{
    g_extensions = g_array_new(FALSE, TRUE, sizeof(ExtensionMeta));
    g_array_set_clear_func(g_extensions, extension_meta_clear);

    if (!extensions_dir || !g_file_test(extensions_dir, G_FILE_TEST_IS_DIR)) {
        g_message("Extensions: directory not found (%s), skipping",
                  extensions_dir ? extensions_dir : "not set");
        return;
    }

    GError *error = NULL;
    GDir *dir = g_dir_open(extensions_dir, 0, &error);
    if (!dir) {
        g_warning("Extensions: cannot open directory: %s", error->message);
        g_error_free(error);
        return;
    }

    const gchar *entry;
    while ((entry = g_dir_read_name(dir)) != NULL) {
        gchar *ext_dir = g_build_filename(extensions_dir, entry, NULL);
        if (!g_file_test(ext_dir, G_FILE_TEST_IS_DIR)) {
            g_free(ext_dir);
            continue;
        }

        /* Parse manifest.json */
        gchar *manifest_path = g_build_filename(ext_dir, "manifest.json", NULL);
        gchar *manifest_text = read_text_file(manifest_path);
        g_free(manifest_path);

        if (!manifest_text) {
            g_warning("Extension '%s': missing manifest.json, skipping", entry);
            g_free(ext_dir);
            continue;
        }

        JsonParser *parser = json_parser_new();
        if (!json_parser_load_from_data(parser, manifest_text, -1, &error)) {
            g_warning("Extension '%s': invalid JSON: %s", entry, error->message);
            g_clear_error(&error);
            g_free(manifest_text);
            g_object_unref(parser);
            g_free(ext_dir);
            continue;
        }
        g_free(manifest_text);

        JsonNode *root = json_parser_get_root(parser);
        if (!root || !JSON_NODE_HOLDS_OBJECT(root)) {
            g_warning("Extension '%s': manifest is not a JSON object, skipping", entry);
            g_object_unref(parser);
            g_free(ext_dir);
            continue;
        }

        JsonObject *obj = json_node_get_object(root);
        if (!json_object_has_member(obj, "name") ||
            !json_object_has_member(obj, "version")) {
            g_warning("Extension '%s': missing 'name' or 'version', skipping", entry);
            g_object_unref(parser);
            g_free(ext_dir);
            continue;
        }

        const gchar *name = json_object_get_string_member(obj, "name");
        const gchar *version = json_object_get_string_member(obj, "version");

        gchar *disabled_path = g_build_filename(ext_dir, ".disabled", NULL);
        gboolean is_disabled = g_file_test(disabled_path, G_FILE_TEST_EXISTS);
        g_free(disabled_path);

        gchar **scripts = json_object_has_member(obj, "scripts")
            ? json_array_to_strv(json_object_get_array_member(obj, "scripts"))
            : NULL;
        gchar **styles = json_object_has_member(obj, "styles")
            ? json_array_to_strv(json_object_get_array_member(obj, "styles"))
            : NULL;

        /* For enabled extensions, validate all referenced files exist */
        if (!is_disabled &&
            (!validate_files(ext_dir, scripts) ||
             !validate_files(ext_dir, styles))) {
            g_warning("Extension '%s': referenced files missing, skipping", entry);
            g_strfreev(scripts);
            g_strfreev(styles);
            g_object_unref(parser);
            g_free(ext_dir);
            continue;
        }

        ExtensionMeta meta = {
            .name = g_strdup(name),
            .version = g_strdup(version),
            .dir_path = ext_dir, /* takes ownership */
            .scripts = scripts,
            .styles = styles,
            .enabled = !is_disabled
        };
        g_array_append_val(g_extensions, meta);

        g_message("Extension '%s' v%s: %s", name, version,
                  is_disabled ? "disabled" : "loaded");
        g_object_unref(parser);
    }

    g_dir_close(dir);
}

/* ---- Overlay setup ---- */

static const gchar OVERLAY_SCRIPT_TEMPLATE[] =
    "(function(){\n"
    "  if(document.getElementById('__kiosk-overlay'))return;\n"
    "  var o=document.createElement('div');\n"
    "  o.id='__kiosk-overlay';\n"
    "  document.documentElement.appendChild(o);\n"
    "  var sr=o.attachShadow({mode:'open'});\n"
    "  var s=document.createElement('style');\n"
    "  s.textContent=':host{position:fixed;top:0;left:0;width:100%%;height:100%%;pointer-events:none;z-index:2147483647;}';\n"
    "  sr.appendChild(s);\n"
    "  window.__kiosk={\n"
    "    overlay:sr,\n"
    "    sendMessage:function(t,p){\n"
    "      return window.webkit.messageHandlers.__kiosk.postMessage(\n"
    "        JSON.stringify({type:t,data:p})\n"
    "      );\n"
    "    },\n"
    "    extensions:%s\n"
    "  };\n"
    "})();\n";

static gchar *build_extensions_json(void)
{
    GString *json = g_string_new("[");
    gboolean first = TRUE;
    for (guint i = 0; i < g_extensions->len; i++) {
        ExtensionMeta *m = &g_array_index(g_extensions, ExtensionMeta, i);
        if (!m->enabled) continue;
        if (!first) g_string_append_c(json, ',');
        gchar *ename = g_strescape(m->name, NULL);
        gchar *ever = g_strescape(m->version, NULL);
        g_string_append_printf(json,
            "{\"name\":\"%s\",\"version\":\"%s\"}", ename, ever);
        g_free(ename);
        g_free(ever);
        first = FALSE;
    }
    g_string_append_c(json, ']');
    return g_string_free(json, FALSE);
}

static void append_json_comma(GString *json)
{
    if (json->len > 1 && json->str[json->len - 1] != '{')
        g_string_append_c(json, ',');
}

static void read_cpu_stats(GString *json)
{
    gchar *stat_text = NULL;
    if (!g_file_get_contents("/proc/stat", &stat_text, NULL, NULL))
        return;
    gchar *eol = strchr(stat_text, '\n');
    if (eol) *eol = '\0';
    gchar *escaped = g_strescape(stat_text, NULL);
    append_json_comma(json);
    g_string_append_printf(json, "\"cpuLine\":\"%s\"", escaped);
    g_free(escaped);
    g_free(stat_text);
}

static void read_memory_stats(GString *json)
{
    gchar *mem_text = NULL;
    if (!g_file_get_contents("/proc/meminfo", &mem_text, NULL, NULL))
        return;
    long mem_total = 0, mem_available = 0, swap_total = 0, swap_free = 0;
    gchar **lines = g_strsplit(mem_text, "\n", -1);
    for (int i = 0; lines[i]; i++) {
        if (g_str_has_prefix(lines[i], "MemTotal:"))
            sscanf(lines[i], "MemTotal: %ld", &mem_total);
        else if (g_str_has_prefix(lines[i], "MemAvailable:"))
            sscanf(lines[i], "MemAvailable: %ld", &mem_available);
        else if (g_str_has_prefix(lines[i], "SwapTotal:"))
            sscanf(lines[i], "SwapTotal: %ld", &swap_total);
        else if (g_str_has_prefix(lines[i], "SwapFree:"))
            sscanf(lines[i], "SwapFree: %ld", &swap_free);
    }
    g_strfreev(lines);
    g_free(mem_text);
    append_json_comma(json);
    g_string_append_printf(json,
        "\"memTotalKB\":%ld,\"memAvailableKB\":%ld,"
        "\"swapTotalKB\":%ld,\"swapFreeKB\":%ld",
        mem_total, mem_available, swap_total, swap_free);
}

static void read_uptime(GString *json)
{
    gchar *text = NULL;
    if (!g_file_get_contents("/proc/uptime", &text, NULL, NULL))
        return;
    double uptime_sec = 0;
    sscanf(text, "%lf", &uptime_sec);
    g_free(text);
    append_json_comma(json);
    g_string_append_printf(json, "\"uptimeSec\":%.1f", uptime_sec);
}

static void read_load_average(GString *json)
{
    gchar *text = NULL;
    if (!g_file_get_contents("/proc/loadavg", &text, NULL, NULL))
        return;
    double load1 = 0, load5 = 0, load15 = 0;
    sscanf(text, "%lf %lf %lf", &load1, &load5, &load15);
    g_free(text);
    append_json_comma(json);
    g_string_append_printf(json,
        "\"loadAvg\":[%.2f,%.2f,%.2f]", load1, load5, load15);
}

static void read_temperature(GString *json)
{
    append_json_comma(json);
    g_string_append(json, "\"temperatures\":[");
    gboolean first = TRUE;

    GDir *dir = g_dir_open("/sys/class/thermal", 0, NULL);
    if (!dir) {
        g_string_append_c(json, ']');
        return;
    }

    const gchar *entry;
    while ((entry = g_dir_read_name(dir)) != NULL) {
        if (!g_str_has_prefix(entry, "thermal_zone"))
            continue;

        gchar *temp_path = g_build_filename("/sys/class/thermal", entry, "temp", NULL);
        gchar *type_path = g_build_filename("/sys/class/thermal", entry, "type", NULL);
        gchar *temp_text = NULL, *type_text = NULL;

        if (g_file_get_contents(temp_path, &temp_text, NULL, NULL)) {
            long millideg = atol(temp_text);
            g_file_get_contents(type_path, &type_text, NULL, NULL);
            if (type_text) g_strstrip(type_text);

            if (!first) g_string_append_c(json, ',');
            g_string_append_printf(json, "{\"zone\":\"%s\",\"type\":\"%s\",\"tempC\":%.1f}",
                                   entry,
                                   type_text ? type_text : "unknown",
                                   millideg / 1000.0);
            first = FALSE;
        }

        g_free(temp_path);
        g_free(type_path);
        g_free(temp_text);
        g_free(type_text);
    }
    g_dir_close(dir);
    g_string_append_c(json, ']');
}

static GHashTable *collect_ipv4_addresses(void)
{
    GHashTable *addrs = g_hash_table_new_full(g_str_hash, g_str_equal, g_free, g_free);
    struct ifaddrs *ifap = NULL;
    if (getifaddrs(&ifap) != 0)
        return addrs;
    for (struct ifaddrs *ifa = ifap; ifa; ifa = ifa->ifa_next) {
        if (!ifa->ifa_addr || ifa->ifa_addr->sa_family != AF_INET)
            continue;
        if (g_strcmp0(ifa->ifa_name, "lo") == 0)
            continue;
        char buf[INET_ADDRSTRLEN];
        struct sockaddr_in *sa = (struct sockaddr_in *)ifa->ifa_addr;
        if (inet_ntop(AF_INET, &sa->sin_addr, buf, sizeof(buf)))
            g_hash_table_insert(addrs, g_strdup(ifa->ifa_name), g_strdup(buf));
    }
    freeifaddrs(ifap);
    return addrs;
}

static void read_network_stats(GString *json)
{
    gchar *text = NULL;
    if (!g_file_get_contents("/proc/net/dev", &text, NULL, NULL))
        return;

    GHashTable *ip_addrs = collect_ipv4_addresses();

    append_json_comma(json);
    g_string_append(json, "\"network\":[");
    gboolean first = TRUE;

    gchar **lines = g_strsplit(text, "\n", -1);
    /* Skip first 2 header lines */
    for (int i = 2; lines[i] && lines[i][0]; i++) {
        gchar *line = g_strstrip(lines[i]);
        gchar *colon = strchr(line, ':');
        if (!colon) continue;

        *colon = '\0';
        gchar *iface = g_strstrip(line);
        gchar *rest = colon + 1;

        /* Skip loopback */
        if (g_strcmp0(iface, "lo") == 0) continue;

        long long rx_bytes = 0, tx_bytes = 0;
        long long rx_packets = 0, tx_packets = 0;
        long long dummy;
        sscanf(rest, "%lld %lld %lld %lld %lld %lld %lld %lld %lld %lld",
               &rx_bytes, &rx_packets, &dummy, &dummy, &dummy, &dummy, &dummy, &dummy,
               &tx_bytes, &tx_packets);

        if (!first) g_string_append_c(json, ',');
        gchar *esc_iface = g_strescape(iface, NULL);
        const gchar *ipv4 = g_hash_table_lookup(ip_addrs, iface);
        g_string_append_printf(json,
            "{\"iface\":\"%s\",\"rxBytes\":%lld,\"txBytes\":%lld,"
            "\"rxPackets\":%lld,\"txPackets\":%lld",
            esc_iface, rx_bytes, tx_bytes, rx_packets, tx_packets);
        if (ipv4) {
            gchar *esc_ip = g_strescape(ipv4, NULL);
            g_string_append_printf(json, ",\"ipv4\":\"%s\"", esc_ip);
            g_free(esc_ip);
        }
        g_string_append_c(json, '}');
        g_free(esc_iface);
        first = FALSE;
    }
    g_strfreev(lines);
    g_free(text);
    g_hash_table_destroy(ip_addrs);
    g_string_append_c(json, ']');
}

static void read_disk_stats(GString *json)
{
    const gchar *mount_points[] = {"/", "/tmp", NULL};

    append_json_comma(json);
    g_string_append(json, "\"disk\":[");
    gboolean first = TRUE;

    for (int i = 0; mount_points[i]; i++) {
        struct statvfs st;
        if (statvfs(mount_points[i], &st) != 0) continue;

        unsigned long long total = (unsigned long long)st.f_blocks * st.f_frsize;
        unsigned long long avail = (unsigned long long)st.f_bavail * st.f_frsize;

        if (!first) g_string_append_c(json, ',');
        g_string_append_printf(json,
            "{\"mount\":\"%s\",\"totalBytes\":%llu,\"availBytes\":%llu}",
            mount_points[i], total, avail);
        first = FALSE;
    }
    g_string_append_c(json, ']');
}

static void read_gpu_freq(GString *json)
{
    append_json_comma(json);
    g_string_append(json, "\"gpu\":{");
    gboolean has_data = FALSE;

    /* Intel GPU frequency */
    gchar *freq_text = NULL;
    if (g_file_get_contents("/sys/class/drm/card0/gt_cur_freq_mhz", &freq_text, NULL, NULL)) {
        g_strstrip(freq_text);
        g_string_append_printf(json, "\"freqMHz\":%s", freq_text);
        g_free(freq_text);
        has_data = TRUE;
    }

    gchar *max_text = NULL;
    if (g_file_get_contents("/sys/class/drm/card0/gt_max_freq_mhz", &max_text, NULL, NULL)) {
        g_strstrip(max_text);
        if (has_data) g_string_append_c(json, ',');
        g_string_append_printf(json, "\"maxFreqMHz\":%s", max_text);
        g_free(max_text);
    }

    g_string_append_c(json, '}');
}

static void read_process_stats(GString *json)
{
    gchar *status_text = NULL;
    if (!g_file_get_contents("/proc/self/status", &status_text, NULL, NULL))
        return;

    long vm_rss = 0, vm_size = 0, threads = 0;
    gchar **lines = g_strsplit(status_text, "\n", -1);
    for (int i = 0; lines[i]; i++) {
        if (g_str_has_prefix(lines[i], "VmRSS:"))
            sscanf(lines[i], "VmRSS: %ld", &vm_rss);
        else if (g_str_has_prefix(lines[i], "VmSize:"))
            sscanf(lines[i], "VmSize: %ld", &vm_size);
        else if (g_str_has_prefix(lines[i], "Threads:"))
            sscanf(lines[i], "Threads: %ld", &threads);
    }
    g_strfreev(lines);
    g_free(status_text);

    append_json_comma(json);
    g_string_append_printf(json,
        "\"process\":{\"vmRssKB\":%ld,\"vmSizeKB\":%ld,\"threads\":%ld,\"pid\":%d}",
        vm_rss, vm_size, threads, getpid());
}

static gchar *read_system_stats(void)
{
    GString *json = g_string_new("{");

    read_cpu_stats(json);
    read_memory_stats(json);
    read_uptime(json);
    read_load_average(json);
    read_temperature(json);
    read_network_stats(json);
    read_disk_stats(json);
    read_gpu_freq(json);
    read_process_stats(json);

    g_string_append_c(json, '}');
    return g_string_free(json, FALSE);
}

static gboolean on_script_message(WebKitUserContentManager *manager,
                                  JSCValue *value,
                                  WebKitScriptMessageReply *reply,
                                  gpointer user_data)
{
    (void)manager; (void)user_data;

    if (!jsc_value_is_string(value)) {
        webkit_script_message_reply_return_error_message(reply, "expected string");
        return TRUE;
    }

    gchar *str = jsc_value_to_string(value);

    /* Parse message type */
    JsonParser *parser = json_parser_new();
    const gchar *type = NULL;
    if (json_parser_load_from_data(parser, str, -1, NULL)) {
        JsonNode *root = json_parser_get_root(parser);
        if (root && JSON_NODE_HOLDS_OBJECT(root)) {
            JsonObject *obj = json_node_get_object(root);
            if (json_object_has_member(obj, "type"))
                type = json_object_get_string_member(obj, "type");
        }
    }

    JSCContext *ctx = jsc_value_get_context(value);

    if (g_strcmp0(type, "getStats") == 0) {
        gchar *stats = read_system_stats();

        /* Inject WebKit page info (needs g_web_view access) */
        GString *full = g_string_new(stats);
        /* Replace closing brace with webkit data */
        g_string_truncate(full, full->len - 1); /* remove '}' */
        if (g_web_view) {
            const gchar *uri = webkit_web_view_get_uri(g_web_view);
            const gchar *title = webkit_web_view_get_title(g_web_view);
            gdouble progress = webkit_web_view_get_estimated_load_progress(g_web_view);
            gchar *esc_uri = uri ? g_strescape(uri, NULL) : g_strdup("");
            gchar *esc_title = title ? g_strescape(title, NULL) : g_strdup("");
            g_string_append_printf(full,
                ",\"webkit\":{\"uri\":\"%s\",\"title\":\"%s\",\"loadProgress\":%.2f}",
                esc_uri, esc_title, progress);
            g_free(esc_uri);
            g_free(esc_title);
        }
        g_string_append_c(full, '}');

        gchar *result_str = g_string_free(full, FALSE);
        JSCValue *result = jsc_value_new_string(ctx, result_str);
        webkit_script_message_reply_return_value(reply, result);
        g_object_unref(result);
        g_free(result_str);
        g_free(stats);
    } else {
        g_message("Extension message: %s", str);
        JSCValue *result = jsc_value_new_string(ctx, "ok");
        webkit_script_message_reply_return_value(reply, result);
        g_object_unref(result);
    }

    g_free(str);
    g_object_unref(parser);
    return TRUE;
}

static void setup_overlay(WebKitUserContentManager *manager)
{
    /* Base overlay script with extensions metadata */
    gchar *ext_json = build_extensions_json();
    gchar *script_src = g_strdup_printf(OVERLAY_SCRIPT_TEMPLATE, ext_json);
    g_free(ext_json);

    WebKitUserScript *script = webkit_user_script_new(
        script_src, WEBKIT_USER_CONTENT_INJECT_TOP_FRAME,
        WEBKIT_USER_SCRIPT_INJECT_AT_DOCUMENT_END, NULL, NULL);
    webkit_user_content_manager_add_script(manager, script);
    webkit_user_script_unref(script);
    g_free(script_src);

    /* Message handler for JS-to-native communication (reply-based) */
    webkit_user_content_manager_register_script_message_handler_with_reply(
        manager, "__kiosk", NULL);
    g_signal_connect(manager, "script-message-with-reply-received::__kiosk",
                     G_CALLBACK(on_script_message), NULL);
}

/* ---- Register extension content on UserContentManager ---- */

static void register_extension_content(WebKitUserContentManager *manager)
{
    for (guint i = 0; i < g_extensions->len; i++) {
        ExtensionMeta *m = &g_array_index(g_extensions, ExtensionMeta, i);
        if (!m->enabled) continue;

        if (m->styles) {
            for (int j = 0; m->styles[j]; j++) {
                gchar *path = g_build_filename(m->dir_path, m->styles[j], NULL);
                gchar *css = read_text_file(path);
                g_free(path);
                if (!css) continue;
                /* Inject CSS into shadow root via JS */
                gchar *escaped = g_strescape(css, NULL);
                gchar *inject_js = g_strdup_printf(
                    "(function(){var k=window.__kiosk;"
                    "if(!k||!k.overlay)return;"
                    "var s=document.createElement('style');"
                    "s.textContent=\"%s\";"
                    "k.overlay.appendChild(s);"
                    "})();", escaped);
                WebKitUserScript *css_script = webkit_user_script_new(
                    inject_js, WEBKIT_USER_CONTENT_INJECT_TOP_FRAME,
                    WEBKIT_USER_SCRIPT_INJECT_AT_DOCUMENT_END, NULL, NULL);
                webkit_user_content_manager_add_script(manager, css_script);
                webkit_user_script_unref(css_script);
                g_free(inject_js);
                g_free(escaped);
                g_free(css);
            }
        }

        if (m->scripts) {
            for (int j = 0; m->scripts[j]; j++) {
                gchar *path = g_build_filename(m->dir_path, m->scripts[j], NULL);
                gchar *js = read_text_file(path);
                g_free(path);
                if (!js) continue;
                WebKitUserScript *s = webkit_user_script_new(
                    js, WEBKIT_USER_CONTENT_INJECT_TOP_FRAME,
                    WEBKIT_USER_SCRIPT_INJECT_AT_DOCUMENT_END, NULL, NULL);
                webkit_user_content_manager_add_script(manager, s);
                webkit_user_script_unref(s);
                g_free(js);
            }
        }
    }
}

/* ---- D-Bus interface ---- */

static const gchar introspection_xml[] =
    "<node>"
    "  <interface name='com.wpe.Kiosk'>"
    "    <method name='Open'>"
    "      <arg type='s' name='url' direction='in'/>"
    "    </method>"
    "    <method name='Reload'/>"
    "    <method name='GetUrl'>"
    "      <arg type='s' name='url' direction='out'/>"
    "    </method>"
    "    <method name='ClearData'>"
    "      <arg type='s' name='scope' direction='in'/>"
    "    </method>"
    "    <method name='ListExtensions'>"
    "      <arg type='a(ssb)' name='extensions' direction='out'/>"
    "    </method>"
    "  </interface>"
    "</node>";

static void on_clear_data_finished(GObject *source, GAsyncResult *result,
                                   gpointer user_data)
{
    GDBusMethodInvocation *invocation = (GDBusMethodInvocation *)user_data;
    GError *error = NULL;

    webkit_website_data_manager_clear_finish(
        WEBKIT_WEBSITE_DATA_MANAGER(source), result, &error);

    if (error) {
        g_dbus_method_invocation_return_gerror(invocation, error);
        g_error_free(error);
    } else {
        if (g_web_view)
            webkit_web_view_reload(g_web_view);
        g_dbus_method_invocation_return_value(invocation, NULL);
    }
}

static void handle_method_call(GDBusConnection *conn,
                               const gchar *sender,
                               const gchar *object_path,
                               const gchar *interface_name,
                               const gchar *method_name,
                               GVariant *parameters,
                               GDBusMethodInvocation *invocation,
                               gpointer user_data)
{
    (void)conn; (void)sender; (void)object_path;
    (void)interface_name; (void)user_data;

    if (g_strcmp0(method_name, "Open") == 0) {
        const gchar *url = NULL;
        g_variant_get(parameters, "(&s)", &url);
        if (url && g_web_view)
            webkit_web_view_load_uri(g_web_view, url);
        g_dbus_method_invocation_return_value(invocation, NULL);
    } else if (g_strcmp0(method_name, "Reload") == 0) {
        if (g_web_view)
            webkit_web_view_reload(g_web_view);
        g_dbus_method_invocation_return_value(invocation, NULL);
    } else if (g_strcmp0(method_name, "GetUrl") == 0) {
        const gchar *url = g_web_view
            ? webkit_web_view_get_uri(g_web_view) : "";
        g_dbus_method_invocation_return_value(
            invocation, g_variant_new("(s)", url ? url : ""));
    } else if (g_strcmp0(method_name, "ClearData") == 0) {
        const gchar *scope = NULL;
        g_variant_get(parameters, "(&s)", &scope);

        WebKitWebsiteDataTypes types = 0;
        if (g_strcmp0(scope, "cache") == 0) {
            types = WEBKIT_WEBSITE_DATA_DISK_CACHE
                  | WEBKIT_WEBSITE_DATA_MEMORY_CACHE;
        } else if (g_strcmp0(scope, "cookies") == 0) {
            types = WEBKIT_WEBSITE_DATA_COOKIES;
        } else if (g_strcmp0(scope, "all") == 0) {
            types = WEBKIT_WEBSITE_DATA_MEMORY_CACHE
                  | WEBKIT_WEBSITE_DATA_DISK_CACHE
                  | WEBKIT_WEBSITE_DATA_OFFLINE_APPLICATION_CACHE
                  | WEBKIT_WEBSITE_DATA_SESSION_STORAGE
                  | WEBKIT_WEBSITE_DATA_LOCAL_STORAGE
                  | WEBKIT_WEBSITE_DATA_COOKIES
                  | WEBKIT_WEBSITE_DATA_DEVICE_ID_HASH_SALT
                  | WEBKIT_WEBSITE_DATA_HSTS_CACHE
                  | WEBKIT_WEBSITE_DATA_ITP
                  | WEBKIT_WEBSITE_DATA_SERVICE_WORKER_REGISTRATIONS
                  | WEBKIT_WEBSITE_DATA_DOM_CACHE;
        } else {
            g_dbus_method_invocation_return_dbus_error(invocation,
                "com.wpe.Kiosk.Error.InvalidScope",
                "Scope must be 'cache', 'cookies', or 'all'");
            return;
        }

        if (!g_session) {
            g_dbus_method_invocation_return_dbus_error(invocation,
                "com.wpe.Kiosk.Error.NotReady",
                "Kiosk session not initialized");
            return;
        }

        WebKitWebsiteDataManager *manager =
            webkit_network_session_get_website_data_manager(g_session);
        webkit_website_data_manager_clear(manager, types, 0, NULL,
                                          on_clear_data_finished, invocation);
    } else if (g_strcmp0(method_name, "ListExtensions") == 0) {
        GVariantBuilder builder;
        g_variant_builder_init(&builder, G_VARIANT_TYPE("a(ssb)"));
        if (g_extensions) {
            for (guint i = 0; i < g_extensions->len; i++) {
                ExtensionMeta *m = &g_array_index(g_extensions, ExtensionMeta, i);
                g_variant_builder_add(&builder, "(ssb)",
                                      m->name, m->version, m->enabled);
            }
        }
        g_dbus_method_invocation_return_value(
            invocation, g_variant_new("(a(ssb))", &builder));
    }
}

static const GDBusInterfaceVTable vtable = {
    .method_call = handle_method_call,
};

static void on_bus_acquired(GDBusConnection *conn, const gchar *name,
                            gpointer user_data)
{
    (void)name; (void)user_data;

    GError *error = NULL;
    GDBusNodeInfo *node = g_dbus_node_info_new_for_xml(introspection_xml, &error);
    if (!node) {
        g_warning("D-Bus introspection parse error: %s", error->message);
        g_error_free(error);
        return;
    }

    g_dbus_connection_register_object(conn, "/", node->interfaces[0],
                                      &vtable, NULL, NULL, &error);
    if (error) {
        g_warning("D-Bus register error: %s", error->message);
        g_error_free(error);
    }

    g_dbus_node_info_unref(node);
}

/* ---- Event handlers ---- */

static void on_web_process_terminated(WebKitWebView *view,
                                      WebKitWebProcessTerminationReason reason,
                                      gpointer data)
{
    (void)data;
    const char *desc = "unknown";
    switch (reason) {
    case WEBKIT_WEB_PROCESS_CRASHED:
        desc = "crashed"; break;
    case WEBKIT_WEB_PROCESS_EXCEEDED_MEMORY_LIMIT:
        desc = "exceeded memory limit"; break;
    case WEBKIT_WEB_PROCESS_TERMINATED_BY_API:
        desc = "terminated by API"; break;
    }
    g_warning("Web process %s, reloading...", desc);
    webkit_web_view_reload(view);
}

/* ---- Application ---- */

static void activate(GApplication *app, gpointer user_data)
{
    const char *url = (const char *)user_data;

    g_application_hold(app);

    g_session = webkit_network_session_new(NULL, NULL);

    WebKitSettings *settings = webkit_settings_new_with_settings(
        "enable-developer-extras", FALSE,
        "enable-webgl", TRUE,
        NULL);

    WebKitUserContentManager *content_manager =
        webkit_user_content_manager_new();

    /* Hide cursor if configured */
    const char *cursor_visible = getenv("WPE_KIOSK_CURSOR_VISIBLE");
    if (cursor_visible && strcmp(cursor_visible, "false") == 0) {
        WebKitUserStyleSheet *sheet = webkit_user_style_sheet_new(
            "* { cursor: none !important; }",
            WEBKIT_USER_CONTENT_INJECT_ALL_FRAMES,
            WEBKIT_USER_STYLE_LEVEL_USER, NULL, NULL);
        webkit_user_content_manager_add_style_sheet(content_manager, sheet);
        webkit_user_style_sheet_unref(sheet);
    }

    /* Load extensions */
    const char *ext_dir = getenv("WPE_KIOSK_EXTENSIONS_DIR");
    scan_extensions(ext_dir);
    setup_overlay(content_manager);
    register_extension_content(content_manager);

    WebKitWebView *view = WEBKIT_WEB_VIEW(g_object_new(WEBKIT_TYPE_WEB_VIEW,
        "network-session", g_session,
        "settings", settings,
        "user-content-manager", content_manager,
        NULL));

    g_object_unref(settings);
    g_web_view = view;

    g_signal_connect(view, "web-process-terminated",
                     G_CALLBACK(on_web_process_terminated), NULL);

    WPEView *wpe_view = webkit_web_view_get_wpe_view(view);
    if (wpe_view) {
        WPEToplevel *toplevel = wpe_view_get_toplevel(wpe_view);
        wpe_toplevel_fullscreen(toplevel);
        wpe_toplevel_set_title(toplevel, "WPE Kiosk");
    }

    webkit_web_view_load_uri(view, url);

    g_bus_own_name(G_BUS_TYPE_SYSTEM,
                   "com.wpe.Kiosk",
                   G_BUS_NAME_OWNER_FLAGS_NONE,
                   on_bus_acquired, NULL, NULL, NULL, NULL);
}

int main(int argc, char *argv[])
{
    const char *url = argc > 1 ? argv[1] : "https://wpewebkit.org";

    GApplication *app = g_application_new("com.wpe.Kiosk",
                                           G_APPLICATION_NON_UNIQUE);
    g_signal_connect(app, "activate", G_CALLBACK(activate), (gpointer)url);

    int status = g_application_run(app, 0, NULL);

    if (g_extensions)
        g_array_unref(g_extensions);
    if (g_session)
        g_object_unref(g_session);
    g_object_unref(app);
    return status;
}
