package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/perses/perses/go-sdk/dashboard"
	"github.com/perses/perses/go-sdk/panel"
	panelgroup "github.com/perses/perses/go-sdk/panel-group"
	listvariable "github.com/perses/perses/go-sdk/variable/list-variable"
	textvariable "github.com/perses/perses/go-sdk/variable/text-variable"
	logstable "github.com/perses/plugins/logstable/sdk/go"
	lokiquery "github.com/perses/plugins/loki/sdk/go/query/log"
	staticlist "github.com/perses/plugins/staticlistvariable/sdk/go"
)

const (
	datasourceName = "loki-audit-datasource"

	// OTLP query: openshift_log_source is a stream label (indexed), avoiding post-filter scan.
	// Audit fields still require | json until structured metadata extraction lands upstream.
	auditLogQueryOTLP = `{log_type="audit", openshift_log_source="kubeAPI"} | json | user_username!~"${exclude_sa}" | user_username=~"(?i).*${username}.*" | verb=~"${verb}" | objectRef_resource=~".*${resource}.*" | objectRef_resource!~"${exclude_resource}" | objectRef_namespace=~".*${namespace}.*" | objectRef_name=~"(?i).*${resource_name}.*" | responseStatus_code=~"${response_code}" | userAgent=~"(?i).*${client}.*" | line_format "User={{.user_username}} | Verb={{.verb}} | Namespace={{.objectRef_namespace}} | Resource={{.objectRef_resource}} | Resource Name={{.objectRef_name}} | Status={{.responseStatus_code}} | Client={{.userAgent}}" ${filter}`

	excludeSACustomAllValue = "system:serviceaccount:.*|system:node:.*|system:kube.*|system:openshift.*|system:apiserver.*|system:aggregator.*|system:open-cluster-management:.*|system:ovn-node:.*|system:authenticated.*|system:unauthenticated.*|system:monitoring.*|system:master.*|system:multus.*"

	excludeResourceCustomAllValue = "events|endpoints|endpointslices|leases|tokenreviews|subjectaccessreviews|selfsubjectaccessreviews|selfsubjectrulesreviews"
)

func main() {
	builder, err := dashboard.New("ocp-audit-log-viewer-otlp",
		dashboard.ProjectName("perses-dev"),
		dashboard.Name("OCP Audit Log Viewer (OTLP)"),
		dashboard.DurationAsString("1h"),
		dashboard.RefreshIntervalAsString("0s"),

		dashboard.AddVariable("username",
			textvariable.Text("",
				textvariable.DisplayName("Username"),
				textvariable.Description("Filter by username (partial match, case-insensitive). Supports regex."),
			),
		),
		dashboard.AddVariable("exclude_sa",
			listvariable.List(
				staticlist.StaticList(
					staticlist.Values(
						"^$",
						"system:serviceaccount:.*",
						"system:node:.*",
						"system:kube.*",
						"system:openshift.*",
						"system:apiserver.*",
						"system:aggregator.*",
						"system:open-cluster-management:.*",
						"system:ovn-node:.*",
						"system:authenticated.*",
						"system:unauthenticated.*",
						"system:monitoring.*",
						"system:master.*",
						"system:multus.*",
					),
				),
				listvariable.DisplayName("Exclude System Users"),
				listvariable.Description("Select None to show all users including system accounts"),
				listvariable.AllowAllValue(true),
				listvariable.AllowMultiple(true),
				listvariable.CustomAllValue(excludeSACustomAllValue),
				listvariable.DefaultValue("$__all"),
			),
		),
		dashboard.AddVariable("verb",
			listvariable.List(
				staticlist.StaticList(
					staticlist.Values("create", "update", "patch", "delete", "get", "list"),
				),
				listvariable.DisplayName("Verb"),
				listvariable.Description("Filter by API verb"),
				listvariable.AllowAllValue(true),
				listvariable.AllowMultiple(true),
				listvariable.CustomAllValue(".*"),
				listvariable.DefaultValue("$__all"),
			),
		),
		dashboard.AddVariable("resource",
			textvariable.Text("",
				textvariable.DisplayName("Resource"),
				textvariable.Description("Resource type (e.g. pods, deployments)"),
			),
		),
		dashboard.AddVariable("resource_name",
			textvariable.Text("",
				textvariable.DisplayName("Resource Name"),
				textvariable.Description("Filter by resource name (partial match)"),
			),
		),
		dashboard.AddVariable("namespace",
			textvariable.Text("",
				textvariable.DisplayName("Namespace"),
				textvariable.Description("Filter by namespace"),
			),
		),
		dashboard.AddVariable("response_code",
			listvariable.List(
				staticlist.StaticList(
					staticlist.Values("200", "201", "204", "304", "400", "401", "403", "404", "409", "422", "500", "503"),
				),
				listvariable.DisplayName("Response Code"),
				listvariable.Description("Filter by HTTP response code"),
				listvariable.AllowAllValue(true),
				listvariable.CustomAllValue(".*"),
				listvariable.DefaultValue("$__all"),
			),
		),
		dashboard.AddVariable("exclude_resource",
			listvariable.List(
				staticlist.StaticList(
					staticlist.Values(
						"^$",
						"events",
						"endpoints",
						"endpointslices",
						"leases",
						"tokenreviews",
						"subjectaccessreviews",
						"selfsubjectaccessreviews",
						"selfsubjectrulesreviews",
					),
				),
				listvariable.DisplayName("Exclude Resources"),
				listvariable.Description("Select None to show all resource types"),
				listvariable.AllowAllValue(true),
				listvariable.AllowMultiple(true),
				listvariable.CustomAllValue(excludeResourceCustomAllValue),
				listvariable.DefaultValue("$__all"),
			),
		),
		dashboard.AddVariable("client",
			textvariable.Text("",
				textvariable.DisplayName("Client"),
				textvariable.Description("Filter by user agent (e.g. oc, kubectl, console). Partial match, case-insensitive."),
			),
		),
		dashboard.AddVariable("filter",
			textvariable.Text(`|~ ".*"`,
				textvariable.DisplayName("LogQL Filter"),
				textvariable.Description(`Raw LogQL stage. Examples: |~ "sradco" (include), !~ "sradco" (exclude), | user_username!~"sradco.*"`),
			),
		),

		// Panel
		dashboard.AddPanelGroup("Audit Events",
			panelgroup.AddPanel("Audit Logs",
				logstable.LogsTable(
					logstable.EnableDetails(true),
					logstable.ShowTime(true),
				),
				panel.AddQuery(
					lokiquery.LokiLogQuery(auditLogQueryOTLP,
						lokiquery.Datasource(datasourceName),
					),
				),
			),
		),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error building dashboard: %v\n", err)
		os.Exit(1)
	}

	data, err := json.MarshalIndent(builder.Dashboard, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(data))
}
