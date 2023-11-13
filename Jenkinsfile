pipeline {
  agent any
  environment {
      LOCALBIN="/var/jenkins_home/workspace/bin"
      TMP="/var/jenkins_home/workspace/tmp"
      ARGOCD_FILE="deploy.yaml"
  }
  stages {
    stage('verify make is installed') {
      steps {
        sh 'make --version'
      }
    }
    stage('make tools') {
      steps {
        sh 'make tools'
      }
    }
    stage('make test') {
      steps {
        sh 'make test'
      }
    }
    stage('make build') {
      steps {
        sh 'make docker-build'
      }
    }
    stage('') {
      stpes {
        sh 'make deploy > $TMP/$ARGOCD_FILE'
      }
    }
    stage('git clone repository') {
      steps {
        git branch: 'main', credentialsId: 'jinu-github', url: 'https://github.com/qj0r9j0vc2/test-argocd'
      }
    }
    stage('edit repository') {
      steps {
        sh "rm $ARGOCD_FILE"
        sh 'cp $TMP/$ARGOCD_FILE $ARGOCD_FILE'
      }
    }
    stage('push git') {
      steps {
	sh "git add ."
	sh "git commit -m 'Jenkins update'"
        sh "git push origin main"
      }
    }
    
  }
}


