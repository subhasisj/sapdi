FROM python:3.6

RUN pip3 install tornado==5.0.2
RUN pip3 install scikit-image
RUN pip3 install matplotlib
RUN pip3 install numpy
RUN pip3 install pandas
RUN pip3 install argparse
RUN pip3 install imutils
RUN pip3 install opencv-python
RUN pip3 install keras==2.1.0
RUN pip3 install tensorflow==1.13.1
RUN pip3 install pyarrow
RUN pip3 install ipython


RUN git clone https://github.com/subhasisj/Credit_Analysis_R.git 

RUN echo "maxinbuf 4194304" > /etc/Rserve.conf

RUN groupadd -g 1972 vflow && useradd -g 1972 -u 1972 -m vflow
USER 1972:1972

WORKDIR /Credit_Analysis_R/

