pipeline {
    agent any
    stages {
        stage('build') {
            steps {
                make manifests
                make generate
            }
        }
        stage('test') {
            steps {
                make test
            }
        }
        stage('deploy') {
            steps {
                make deploy
            }
        }
    }
}