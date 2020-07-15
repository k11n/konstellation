import React from 'react';
import clsx from 'clsx';
import Layout from '@theme/Layout';
import Link from '@docusaurus/Link';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import useBaseUrl from '@docusaurus/useBaseUrl';
import styles from './styles.module.css';
import Highlight, { defaultProps } from "prism-react-renderer";
import dracula from 'prism-react-renderer/themes/dracula';

const features = [
  {
    title: <>Easy to use</>,
    imageUrl: 'img/undraw_docusaurus_mountain.svg',
    description: (
      <>
        Konstellation gives you a Heroku-like experience on top of Kubernetes that you fully control. It provides a CLI that manages every aspect of app deployments. New apps are deployed in minutes with minimal configuration.
      </>
    ),
  },
  {
    title: <>Build for microservices</>,
    imageUrl: 'img/undraw_docusaurus_tree.svg',
    description: (
      <>
        Built on years of experience with running services on Kubernetes. Konstellation provides an integrated stack including load balancing, autoscaling, service mesh, release management, and observability.
      </>
    ),
  },
  {
    title: <>Integrated with AWS</>,
    imageUrl: 'img/undraw_docusaurus_react.svg',
    description: (
      <>
        Konstellation has been optimized to work on AWS. It manages EKS clusters, nodepools, VPCs, and load balancers. It integrates with other AWS services to provide a secure and robust apps platform.
      </>
    ),
  },
];

const cliDemo = `% kon cluster create
...
% kon app load myapp.yaml
...
% kon app status myapp
Target: production
Hosts: myapp.mydomain.com
Load Balancer: b0d94a8d-istiosystem-konin-a4cf-358886547.us-west-2.elb.amazonaws.com
Scale: 2 min, 10 max

RELEASE                     BUILD                DATE                   PODS    STATUS    TRAFFIC
myapp-20200423-1531-c495    registry/myapp:3     2020-04-23 15:31:40    2/2     released  100%
myapp-20200421-1228-c495    registry/myapp:2     2020-04-21 12:28:11    0       retired   0%
myapp-20200421-1102-b723    registry/myapp:1     2020-04-21 11:02:03    0       retired   0%`;

const manifestExample = `apiVersion: k11n.dev/v1alpha1
kind: App
metadata:
  name: myapp
spec:
  image: registry/myapp
  ports:
    - name: http
      port: 80
  targets:
    - name: production
      scale: {min: 2, max: 10}
      ingress:
        hosts:
          - myapp.mydomain.com
        port: http`;

function Feature({imageUrl, title, description}) {
  const imgUrl = useBaseUrl(imageUrl);
  return (
    <div className={clsx('col col--4', styles.feature)}>
      {imgUrl && (
        <div className="text--center">
          <img className={styles.featureImage} src={imgUrl} alt={title} />
        </div>
      )}
      <h3>{title}</h3>
      <p>{description}</p>
    </div>
  );
}

function Home() {
  const context = useDocusaurusContext();
  const {siteConfig = {}} = context;
  return (
    <Layout
      title="Home"
      description="Description will go into a meta tag in <head />">
      <header className={clsx('hero hero--primary', styles.heroBanner)}>
        <div className="container">
          <h1 className="hero__title">{siteConfig.title}</h1>
          <p className="hero__subtitle">{siteConfig.tagline}</p>
          <div className={styles.buttons}>
            <Link
              className={clsx(
                'button button--outline button--secondary button--lg',
                styles.getStarted,
              )}
              to={useBaseUrl('docs/')}>
              Get Started
            </Link>
          </div>
        </div>
      </header>
      <main>
        {features && features.length > 0 && (
          <section className={styles.features}>
            <div className="container">
              <div className="row">
                {features.map((props, idx) => (
                  <Feature key={idx} {...props} />
                ))}
              </div>
              <div className="row">
                <div className="col col--12">
                  <br/>
                  <h3>An operator for apps</h3>
                  <hr />
                  <p>
                    Deploying apps on Kubernetes can be really complex. You are
                    forced to deal with raw resources, having to manually plan
                    out the dependencies and references.
                    Konstellation is a layer on top of Kubernetes focused around
                    apps. It comes with a powerful CLI that manages all aspects
                    of your clusters and apps.
                  </p>
                  <pre className="language-text">
                    {cliDemo}
                  </pre>
                </div>
              </div>
              <div className="row">
                <div className="col col--12">
                  <br />
                  <h3>One manifest to rule them all</h3>
                  <hr />
                  <p>
                    Konstellation provides high level custom resources and then manages
                    native Kubernetes resources behind the scenes. This means the end of
                    copying and pasting resource templates that you don't understand.
                    The following app manifest would set up ReplicaSets, Service,
                    Autoscaler, Ingress, along with the necessary resources for the
                    service mesh.
                  </p>
                  <Highlight {...defaultProps} theme={dracula} code={manifestExample} language="yaml">
                    {({ className, style, tokens, getLineProps, getTokenProps }) => (
                      <pre className={className} style={style}>
                        {tokens.map((line, i) => (
                          <div {...getLineProps({ line, key: i })}>
                            {line.map((token, key) => (
                              <span {...getTokenProps({ token, key })} />
                            ))}
                          </div>
                        ))}
                      </pre>
                    )}
                  </Highlight>
                </div>
              </div>
            </div>
          </section>
        )}

      </main>
    </Layout>
  );
}

export default Home;
