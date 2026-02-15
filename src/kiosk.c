/* WPE Kiosk — minimal fullscreen browser using WPEPlatform API */
#include <wpe/webkit.h>
#include <wpe/wpe-platform.h>
#include <gio/gio.h>
#include <stdlib.h>

static WebKitWebView *g_web_view = NULL;

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
    "  </interface>"
    "</node>";

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

    WebKitNetworkSession *session = webkit_network_session_new(NULL, NULL);

    WebKitSettings *settings = webkit_settings_new_with_settings(
        "enable-developer-extras", FALSE,
        "enable-webgl", TRUE,
        NULL);

    WebKitWebView *view = WEBKIT_WEB_VIEW(g_object_new(WEBKIT_TYPE_WEB_VIEW,
        "network-session", session,
        "settings", settings,
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

    g_object_unref(session);
}

int main(int argc, char *argv[])
{
    const char *url = argc > 1 ? argv[1] : "https://wpewebkit.org";

    GApplication *app = g_application_new("com.wpe.Kiosk",
                                           G_APPLICATION_NON_UNIQUE);
    g_signal_connect(app, "activate", G_CALLBACK(activate), (gpointer)url);

    int status = g_application_run(app, 0, NULL);

    g_object_unref(app);
    return status;
}
