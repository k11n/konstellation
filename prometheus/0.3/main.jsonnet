local k = import 'ksonnet/ksonnet.beta.4/k.libsonnet';

local kp =
  (import 'kube-prometheus/kube-prometheus.libsonnet') +
  (import 'kube-prometheus/kube-prometheus-anti-affinity.libsonnet') +
  (import 'kube-prometheus/kube-prometheus-kube-aws.libsonnet') +
  // create cluster role, as 0.3 doesn't support it yet
  {
    prometheus+:: {
        clusterRole+: {
            rules+:
            local role = k.rbac.v1.role;
            local policyRule = role.rulesType;
            local rule = policyRule.new() +
                            policyRule.withApiGroups(['']) +
                            policyRule.withResources([
                            'services',
                            'endpoints',
                            'pods',
                            ]) +
                            policyRule.withVerbs(['get', 'list', 'watch']);
            [rule]
      },
    }
  } +
  {
    _config+:: {
      namespace: 'kon-system',
    },
  };

{
  ['setup/prometheus-operator-' + name]: kp.prometheusOperator[name]
  for name in std.filter((function(name) name != 'serviceMonitor'), std.objectFields(kp.prometheusOperator))
} +
// serviceMonitor is separated so that it can be created after the CRDs are ready
{ 'prometheus-operator-serviceMonitor': kp.prometheusOperator.serviceMonitor } +
{ ['node-exporter-' + name]: kp.nodeExporter[name] for name in std.objectFields(kp.nodeExporter) } +
{ ['kube-state-metrics-' + name]: kp.kubeStateMetrics[name] for name in std.objectFields(kp.kubeStateMetrics) } +
{ ['alertmanager-' + name]: kp.alertmanager[name] for name in std.objectFields(kp.alertmanager) } +
{ ['prometheus-' + name]: kp.prometheus[name] for name in std.objectFields(kp.prometheus) } +
{ ['prometheus-adapter-' + name]: kp.prometheusAdapter[name] for name in std.objectFields(kp.prometheusAdapter) } +
{ ['grafana-' + name]: kp.grafana[name] for name in std.objectFields(kp.grafana) }
