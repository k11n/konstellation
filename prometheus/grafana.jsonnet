{
	_config+:: {
    grafana+:: {
      config: { // http://docs.grafana.org/installation/configuration/
        sections: {
          "auth.anonymous": {enabled: true},
        },
      },
    },
  },
}
