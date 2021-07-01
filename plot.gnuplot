set title 'hashring magic factor selection'
set terminal png size 1024,768
set grid
set xtics (150, 1000, 2000, 3000, 5000, 7000)
set xlabel 'magic factor'
set ylabel 'objects, %'
set y2label 'duration, ms'
set y2tics nomirror
plot \
	'data_sub.csv' using 1:2 axis x1y1 smooth sbezier with lines title 'distribution standard deviation', \
	'data_sub.csv' using 1:4 axis x1y2 smooth sbezier with lines dashtype 3 title 'time to rebuild the whole ring'  
