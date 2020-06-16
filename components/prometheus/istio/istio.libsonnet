{
  prometheus+:: {
    local p = self,
    local namespace = 'istio-system',
    local telemetryNamespace = 'istio-system',
    local scrapeInterval = '30s',
    // service monitor idea from: https://github.com/istio/installer/tree/master/istio-telemetry/prometheus-operator/templates
    serviceMonitorIstioMesh:
      {
        apiVersion: 'monitoring.coreos.com/v1',
        kind: 'ServiceMonitor',
        metadata: {
          name: 'istio-mesh-monitor',
          namespace: namespace,
          labels: {
            'monitoring': 'istio-mesh',
          },
        },
        spec: {
          selector: {
            matchExpressions: [
              {
                key: 'istio',
                operator: 'In',
                values: ['mixer'],
              },
            ]
          },
          namespaceSelector: {
            matchNames: [
              telemetryNamespace,
            ],
          },
          endpoints: [
            {
              port: 'prometheus',
              interval: scrapeInterval,
            },
          ],
        },
      },
    serviceMonitorIstioComponents:
      {
        apiVersion: 'monitoring.coreos.com/v1',
        kind: 'ServiceMonitor',
        metadata: {
          name: 'istio-component-monitor',
          namespace: namespace,
          labels: {
            'monitoring': 'istio-components',
          },
        },
        spec: {
          jobLabel: 'istio',
          selector: {
            matchExpressions: [
              {
                key: 'istio',
                operator: 'In',
                values: ['mixer', 'pilot', 'gallery', 'citadel'],
              },
            ]
          },
          namespaceSelector: {
            any: true
          },
          endpoints: [
            {
              port: 'http-monitoring',
              interval: scrapeInterval,
            },
            {
              port: 'http-policy-monitoring',
              interval: scrapeInterval,
            },
          ],
        },
      },
    serviceMonitorIstioEnvoyStats:
      {
        apiVersion: 'monitoring.coreos.com/v1',
        kind: 'ServiceMonitor',
        metadata: {
          name: 'envoy-stats-monitor',
          namespace: namespace,
          labels: {
            'monitoring': 'istio-proxies',
          },
        },
        spec: {
          selector: {
            matchExpressions: [
              {
                key: 'istio-prometheus-ignore',
                operator: 'DoesNotExist',
              },
            ]
          },
          namespaceSelector: {
            any: true,
          },
          jobLabel: 'envoy-stats',
          endpoints: [
            {
              path: '/stats/prometheus',
              targetPort: 15090,
              interval: scrapeInterval,
              relabelings: [
                {
                  sourceLabels: ['__meta_kubernetes_pod_container_port_name'],
                  action: 'keep',
                  regex: '.*-envoy-prom',
                },
                {
                  action: 'labelmap',
                  regex: '__meta_kubernetes_pod_label_(.+)'
                },
                {
                  sourceLabels: ['__meta_kubernetes_namespace'],
                  action: 'replace',
                  targetLabel: 'namespace',
                },
                {
                  sourceLabels: ['__meta_kubernetes_pod_name'],
                  action: 'replace',
                  targetLabel: 'pod_name',
                },
              ],
            },
          ],
        },
      },
    // serviceMonitorKubernetesPods:
    //   {
    //     apiVersion: 'monitoring.coreos.com/v1',
    //     kind: 'ServiceMonitor',
    //     metadata: {
    //       name: 'kubernetes-pods-monitor',
    //       namespace: namespace,
    //       labels: {
    //         'monitoring': 'kube-pods',
    //       },
    //     },
    //     spec: {
    //       jobLabel: 'kubernetes-pods',
    //       selector: {
    //         matchExpressions: [
    //           {
    //             key: 'istio-prometheus-ignore',
    //             operator: 'DoesNotExist',
    //           },
    //         ]
    //       },
    //       namespaceSelector: {
    //         any: true,
    //       },
    //       endpoints: [
    //         {
    //           interval: scrapeInterval,
    //           relabelings: [
    //             {
    //               sourceLabels: ['__meta_kubernetes_pod_annotation_prometheus_io_scrape'],
    //               action: 'keep',
    //               regex: 'true',
    //             },
    //             {
    //               sourceLabels: ['__meta_kubernetes_pod_annotation_sidecar_istio_io_status', '__meta_kubernetes_pod_annotation_prometheus_io_scheme'],
    //               action: 'keep',
    //               regex: '((;.*)|(.*;http)|(.??))',
    //             },
    //             {
    //               sourceLabels: ['__meta_kubernetes_pod_annotation_istio_mtls'],
    //               action: 'drop',
    //               regex: 'true',
    //             },
    //             {
    //               sourceLabels: ['__meta_kubernetes_pod_annotation_prometheus_io_path'],
    //               action: 'replace',
    //               targetLabel: '__metrics_path__',
    //               regex: '(.+)'
    //             },
    //             {
    //               sourceLabels: ['__address__', '__meta_kubernetes_pod_annotation_prometheus_io_port'],
    //               action: 'replace',
    //               targetLabel: '__address__',
    //               regex: '([^:]+)(?::\d+)?;(\d+)',
    //               replacement: '$1:$2'
    //             },
    //             {
    //               action: 'labelmap'
    //               regex: '__meta_kubernetes_pod_label_(.+)'
    //             },
    //             {
    //               sourceLabels: ['__meta_kubernetes_namespace'],
    //               action: 'replace',
    //               targetLabel: 'namespace',
    //             },
    //             {
    //               sourceLabels: ['__meta_kubernetes_pod_name'],
    //               action: 'replace',
    //               targetLabel: 'pod_name',
    //             },
    //           ],
    //         },
    //       ],
    //     },
    //   },
    podMonitorIstio:
    // pod monitor idea from: https://github.com/coreos/prometheus-operator/issues/2502
      {
        apiVersion: 'monitoring.coreos.com/v1',
        kind: 'PodMonitor',
        metadata: {
          name: "istio-proxy",
          namespace: namespace,
          labels: {
            istio: 'proxy',
          },
        },
        spec: {
          jobLabel: 'component',
          selector: {
            matchLabels: {
              'security.istio.io/tlsMode': "istio"
            },
            matchExpressions: [
              {
                key: 'migration',
                operator: 'NotIn',
                values: [ 'true', '1' ],
              },
            ],
          },

          namespaceSelector: {
            any: true,
          },

          podMetricsEndpoints: [
            {
              port: 'http-envoy-prom',
              path: '/stats/prometheus',
              relabelings: [
                {
                  action: 'labeldrop',
                  regex: '__meta_kubernetes_pod_label_skaffold_dev.*'
                },
                {
                  action: 'labeldrop',
                  regex: '__meta_kubernetes_pod_label_pod_template_hash.*'
                },
                {
                  action: 'labelmap',
                  regex: '__meta_kubernetes_pod_label_(.+)'
                },
              ],
            },
          ],
        },
      },
  }
}
