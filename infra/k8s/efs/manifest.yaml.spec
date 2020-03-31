---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: efs-provisioner
---
kind: Deployment
apiVersion: apps/v1
metadata:
  name: efs-provisioner
  annotations:
    cni: "flannel"
spec:
  replicas: 1
  selector:
    matchLabels:
      app: efs-provisioner
  strategy:
    type: Recreate
  template:
    metadata:
      annotations:
        cni: "flannel"
      labels:
        app: efs-provisioner
    spec:
      serviceAccountName: efs-provisioner
      containers:
        - name: efs-provisioner
          image: quay.io/external_storage/efs-provisioner:latest
          env:
            - name: FILE_SYSTEM_ID
              valueFrom:
                configMapKeyRef:
                  name: efs-provisioner
                  key: file.system.id
            - name: AWS_REGION
              valueFrom:
                configMapKeyRef:
                  name: efs-provisioner
                  key: aws.region
            - name: PROVISIONER_NAME
              valueFrom:
                configMapKeyRef:
                  name: efs-provisioner
                  key: provisioner.name
          volumeMounts:
            - name: pv-volume
              mountPath: /persistentvolumes
      volumes:
        - name: pv-volume
          nfs:
            server: ${EFS_DNSNAME}
            path: /
---
kind: StorageClass
apiVersion: storage.k8s.io/v1
metadata:
  name: aws-efs
provisioner: testground.io/aws-efs
---
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: efs
  annotations:
    volume.beta.kubernetes.io/storage-class: "aws-efs"
spec:
  accessModes:
    - ReadOnlyMany
    - ReadWriteOnce
  storageClassName: aws-efs
  resources:
    requests:
      storage: 1000Mi
---
