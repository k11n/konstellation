import React, { useState } from 'react';
import clsx from 'clsx';
import Layout from '@theme/Layout';
import Link from '@docusaurus/Link';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import useBaseUrl from '@docusaurus/useBaseUrl';
import styles from './styles.module.css';
import Highlight, { defaultProps } from "prism-react-renderer";
import highlightTheme from "prism-react-renderer/themes/nightOwl";
import Lightbox from 'react-image-lightbox';
import 'react-image-lightbox/style.css';
import Typical from 'react-typical'

const featureCols = [
  {
    title: 'Kubernetes made simple',
    // imageUrl: 'img/undraw_docusaurus_mountain.svg',
    description: `Konstellation gives you a Heroku-like experience on top of
      Kubernetes that you fully control. It provides a full featured CLI that
      significantly reduces the complexity of working with Kubernetes.`,
  },
  {
    title: 'Designed for microservices',
    // imageUrl: 'img/undraw_docusaurus_tree.svg',
    description: `Built on years of experience with running services on
      Kubernetes. Konstellation provides an integrated stack including load
      balancing, autoscaling, service discovery, release management, and
      observability.`,
  },
  {
    title: 'Integrated with AWS',
    // imageUrl: 'img/undraw_docusaurus_react.svg',
    description: `Konstellation has been optimized to work on AWS. It manages
      EKS clusters, nodepools, VPCs, and load balancers. It integrates with
      other AWS services to provide a secure and robust apps platform.`,
  },
];

const manifestExample = `apiVersion: k11n.dev/v1alpha1
kind: App
metadata:
  name: app2048
spec:
  image: "alexwhen/docker-2048"
  ports:
    - name: http
      port: 80
  targets:
    - name: production
      scale:
        - min: 2
          max: 5
      ingress:
        hosts:
          - 2048.mydomain.com
        port: http`;

const featureRows = [
  {
    title: 'A new kind of cluster manager',
    descriptions: [
      `Creating Kubernetes clusters can be complex, typically involving a complex
      sequence of steps that can be difficult to reproduce.`,
      `Konstellation is a full-stack cluster manager focused on end to end management.
      It uses Terraform to automate creation (and teardown) of cloud resources.`,
      `Get a fully configured Kubernetes cluster in 15 minutes!`,
    ],
    videoUrl: 'https://konstellation-public.s3-us-west-2.amazonaws.com/cluster-demo-720p.mp4',
  },
  {
    title: 'Apps as Kubernetes resources',
    descriptions: [
      `Konstellation uses custom resource definitions (CRDs) to define Apps as
      a first class resource.`,
      `The app manifest acts as the single source of truth to how an app should
      be deployed.
      If it's public facing, Konstellation will create a load balancer that you
      can point traffic to.`,
      `Konstellation runs as an operator inside Kubernetes that syncs your app
      resource to underlying ReplicaSet, Service, Autoscaler, Ingress, along with the
      necessary resources for the service mesh.`,
    ],
    sectionContent: (
      <Highlight {...defaultProps} theme={highlightTheme} code={manifestExample} language="yaml">
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
      `Getting visibility into how apps are performing is often overlooked,
      but it's a crucial piece of any production deployment.`,
      `Konstellation comes with an app observability stack out of the
      box, with a redundant Prometheus setup and pre-configured Grafana
      dashboards for every app that you deploy.`,
      `It's fully extensible to collect app specific metrics and host custom dashboards.`,
    ],
    imageUrls: [
      'img/screen/home-observe-1.png',
      'img/screen/home-observe-2.png',
      'img/screen/home-observe-3.png',
      'img/screen/home-observe-4.png',
    ],
  },
  {
    title: 'The missing devops toolchain',
    descriptions: [
      `Inspired by devops challenges at Medium, Konstellation provides the full
      set of tools needed to operate production workloads on top of Kubernetes.`,
      `Release management, configuration management, troubleshooting tools,
      rollbacks, cluster backup and replication.
      All accessible via CLI with a secure proxy into the cluster.`,
      `Konstellation is self-contained and runs entirely inside Kubernetes and
      dev machines. There are no hosted dependencies.`
    ],
    sectionContent: (
      <div className="homeShell">
        <Typical
          loop={Infinity}
          steps={[
            'kon app new', 1000,
            'kon app deploy', 1000,
            'kon app halt', 1000,
            'kon app list', 1000,
            'kon app local', 1000,
            'kon app logs', 1000,
            'kon app restart', 1000,
            'kon app rollback', 1000,
            'kon app shell', 1000,
            'kon app status', 1000,
            'kon cluster configure', 1000,
            'kon cluster create', 1000,
            'kon cluster export', 1000,
            'kon cluster import', 1000,
            'kon cluster list', 1000,
            'kon cluster shell', 1000,
            'kon config edit', 1000,
            'kon config list', 1000,
            'kon config delete', 1000,
            'kon launch alertmanager', 1000,
            'kon launch grafana', 1000,
            'kon launch kubedash', 1000,
            'kon launch prometheus', 1000,
            'kon launch proxy', 1000,
            'kon nodepool list', 1000,
            'kon nodepool create', 1000,
            'kon nodepool delete', 1000,
          ]}
        />
      </div>
    ),
  },
  {
    title: 'Kubernetes at its best',
    descriptions: [
      `Konstellation is designed as a component running on top of Kubernetes
      and does not attempt to hide k8s internals.`,
      `Clusters managed by Konstellation are fully compatible with other
      Kubernetes software, you are free to customize as you see fit.`,
      `You retain full control of the underlying cluster and its resources.
      Because Konstellation creates native resources, it's compatible with other
      Kubernetes components and tools.`,
    ],
    imageUrls: [
      'img/screen/home-kubedash.png',
      'img/screen/home-kubepods.png',
      'img/screen/home-kubeservices.png',
    ],
  }
];


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

function FeatureRow({title, descriptions, videoUrl, imageUrls, sectionContent, rowNum}) {
  const [lbOpen, setLbOpen] = useState(false);
  const [imgIdx, setImgIdx] = useState(0);
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
    if (videoUrl !== undefined) {
      sectionContent = (
        <video className="featureVideo" autoPlay={true} muted={true} controls={true}>
          <source type="video/mp4" src={videoUrl}/>
        </video>
      );
    } else if (imageUrls !== undefined && imageUrls.length > 0) {
      sectionContent = (
        <>
          <img className="featureImage" src={imageUrls[imgIdx]} alt={title} onClick={() => setLbOpen(true)}/>
          {lbOpen && (
            <Lightbox
              mainSrc={imageUrls[imgIdx]}
              prevSrc={imageUrls[(imgIdx + imageUrls.length - 1) % imageUrls.length]}
              nextSrc={imageUrls[(imgIdx + 1) % imageUrls.length]}
              onCloseRequest={() => setLbOpen(false)}
              onMovePrevRequest={() =>
                setImgIdx((imgIdx + imageUrls.length - 1) % imageUrls.length)
              }
              onMoveNextRequest={() =>
                setImgIdx((imgIdx + 1) % imageUrls.length)
              }
            />
          )}
        </>
      );
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
          <h1 className="heroTagline">
            The swiss army knife for running Kubernetes in the cloud
          </h1>
          <div className={styles.buttons}>
            <Link
              className={clsx(
                'button button--outline button--secondary buttonCta',
                styles.getStarted,
              )}
              to={useBaseUrl('docs/')}>
              Get Started
            </Link>
          </div>
        </div>
      </header>
      <main>
        {featureCols && featureCols.length > 0 && (
          <section className="featureColumnSection">
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
