import React from 'react';
import clsx from 'clsx';
import Layout from '@theme/Layout';
import Link from '@docusaurus/Link';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import useBaseUrl from '@docusaurus/useBaseUrl';
import styles from './styles.module.css';
import Highlight, { defaultProps } from "prism-react-renderer";
import dracula from 'prism-react-renderer/themes/dracula';

const featureCols = [
  {
    title: <>Easy to use</>,
    imageUrl: 'img/undraw_docusaurus_mountain.svg',
    description: `Konstellation gives you a Heroku-like experience on top of
      Kubernetes that you fully control. It provides a CLI that manages every
      aspect of app deployments. New apps are deployed in minutes with minimal
      configuration.`,
  },
  {
    title: <>Build for microservices</>,
    imageUrl: 'img/undraw_docusaurus_tree.svg',
    description: `Built on years of experience with running services on
      Kubernetes. Konstellation provides an integrated stack including load
      balancing, autoscaling, service discovery, release management, and
      observability.`,
  },
  {
    title: <>Integrated with AWS</>,
    imageUrl: 'img/undraw_docusaurus_react.svg',
    description: `Konstellation has been optimized to work on AWS. It manages
      EKS clusters, nodepools, VPCs, and load balancers. It integrates with
      other AWS services to provide a secure and robust apps platform.`,
  },
];

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
      scale:
        - min: 2
          max: 10
      ingress:
        hosts:
          - myapp.mydomain.com
        port: http`;

const featureRows = [
  {
    title: 'A new kind of cluster manager',
    descriptions: [
      `Creating Kubernetes clusters can be complex, typically involving a complex
      sequence of steps that can be difficult to reproduce.`,
      `Konstellation is a full-stack cluster manager focused on end to end management.
      It uses Terraform to automate creation of cloud resources.`,
      `Get a fully configured Kubernetes cluster in 15 minutes!`,
    ],
    mediaUrl: 'https://konstellation-public.s3-us-west-2.amazonaws.com/cluster-demo-720p.mp4',
    isVideo: true,
  },
  {
    title: 'Apps as Kubernetes resources',
    descriptions: [
      `Konstellation uses custom resource definitions (CRDs) to define Apps as
      a first class resource.`,
      `With a single simple manifest, you can define all there needs to deploying
      your app. Receiving a load balancer address that you can point traffic to.`,
      `It runs as an operator inside Kubernetes and syncs your app resource to
      underlying ReplicaSet, Service, Autoscaler, Ingress, along with the
      necessary resources for the service mesh.`,
    ],
    sectionContent: (
      <Highlight {...defaultProps} code={manifestExample} language="yaml">
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
    ),
  },
  {
    title: 'Observability out of the box',
    descriptions: [
      `Getting visibility into how apps are performing is often overlooked, but it's a critical piece of any production deployment.`,
      `Konstellation comes with full observability out of the box, with a redundant Prometheus setup and pre-configured Grafana dashboards to give you insights.`,
      `It's fully extensible to collect app specific metrics as well.`,
    ],
  },
  {
    title: 'Built for resilience',
    descriptions: [
      `Inspired by devops challenges at Medium, Konstellation is built to scale
      production workloads reliably and predictably.`,
      `Konstellation incorporates advanced tools such as release management,
      rollbacks, cluster backup and replication. The CLI gives you the full
      suite of tools needed to operate production environments.`,
    ]
  }];


function FeatureColumn({imageUrl, title, description}) {
  const imgUrl = useBaseUrl(imageUrl);
  return (
    <div className={clsx('col col--4', styles.feature)}>
      {imgUrl && (
        <div className="text--center">
          <img className={styles.featureImage} src={imgUrl} alt={title} />
        </div>
      )}
      <h2>{title}</h2>
      <p>{description}</p>
    </div>
  );
}

function FeatureRow({title, descriptions, mediaUrl, isVideo, sectionContent, rowNum}) {
  if (rowNum === undefined) {
    rowNum = 0
  }
  const isEven = rowNum % 2 == 0
  const textSection = (
    <div className="col col--6">
      <h2>{title}</h2>
      {descriptions.map((desc, idx) => (
        <p>{desc}</p>
      ))}
    </div>
  )
  if (sectionContent === undefined) {
    if (isVideo) {
      sectionContent = (
        <video className="featureVideo" autoPlay={true} muted={true} controls={true}>
          <source type="video/mp4" src={mediaUrl}/>
        </video>
      );
    } else {
      sectionContent = 'Placeholder';
    }
  }
  const imageSection = (
    <div className="col col--6">
      {sectionContent}
    </div>
  )

  return (
    <section className={isEven ? 'alternateRow' : ''}>
      <div className="container">
        <div className="row homeSection">
          {isEven ? textSection : imageSection}
          {isEven ? imageSection : textSection}
        </div>
      </div>
    </section>
  )
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
          <h1 className="heroTagline">{siteConfig.tagline}</h1>
          {/* <h1 className="hero__title">{siteConfig.title}</h1>
          <p className="hero_tagline">{siteConfig.tagline}</p> */}
          <div className={styles.buttons}>
            <Link
              className={clsx(
                'button button--outline button--secondary buttonCta',
                styles.getStarted,
              )}
              to={useBaseUrl('docs/konstellation/introduction')}>
              Get Started
            </Link>
          </div>
        </div>
      </header>
      <main>
        {featureCols && featureCols.length > 0 && (
          <section className={styles.features}>
            <div className="container">
              <div className="row">
                {featureCols.map((props, idx) => (
                  <FeatureColumn key={idx} {...props} />
                ))}
              </div>
            </div>
          </section>
        )}

        {featureRows.map((props, idx) => (
          <FeatureRow key={idx} rowNum={idx} {...props} />
        ))}
      </main>
    </Layout>
  );
}

export default Home;
